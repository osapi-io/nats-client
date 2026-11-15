// Copyright (c) 2025 John Dewey

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// MsgHandler handles a core NATS message. ctx carries the trace of the
// message, so that the work of the handler is in the trace of the request
// that published it.
type MsgHandler func(ctx context.Context, msg *nats.Msg)

// JetStreamMessageHandler handles a JetStream message, with ctx as for
// MsgHandler. An error leaves the message unacknowledged, so that JetStream
// delivers it again.
type JetStreamMessageHandler func(ctx context.Context, msg jetstream.Msg) error

// ConsumeOptions configures message consumption behavior.
type ConsumeOptions struct {
	// QueueGroup for load balancing across multiple consumers (optional)
	QueueGroup string
	// MaxInFlight limits the number of unacknowledged messages
	MaxInFlight int
}

// subjectAttr is the attribute of the subject of a message, on its span only.
// A subject can name a host, so the metric does not have it.
func subjectAttr(
	subject string,
) attribute.KeyValue {
	return attribute.String("messaging.destination.name", subject)
}

// inject puts the trace context of ctx in header.
func inject(
	ctx context.Context,
	header nats.Header,
) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(http.Header(header)))
}

// publish runs send as the publish of a message to subject, with a producer
// span, and with the trace context of that span in the headers of the
// message. A receiver's span is then a child of the publish.
func (c *Client) publish(
	ctx context.Context,
	name string,
	subject string,
	data []byte,
	send func(context.Context, *nats.Msg) error,
) error {
	o := newOp(name, "", subjectAttr(subject))
	began := time.Now()
	ctx, span := c.tel.start(ctx, o, trace.SpanKindProducer)

	msg := &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
	inject(ctx, msg.Header)
	err := send(ctx, msg)

	c.tel.end(ctx, o, span, began, err)

	return err
}

// Publish publishes data to subject in JetStream. When ctx is inside a trace,
// the headers of the message carry its trace context.
func (c *Client) Publish(
	ctx context.Context,
	subject string,
	data []byte,
) error {
	c.logger.Debug(
		"publishing message",
		slog.String("subject", subject),
		slog.Int("data_size", len(data)),
	)

	if c.ExtJS == nil {
		return fmt.Errorf("jetstream not initialized: call Connect() first")
	}

	return c.publish(ctx, "nats.publish", subject, data,
		func(ctx context.Context, msg *nats.Msg) error {
			if _, err := c.ExtJS.PublishMsg(ctx, msg); err != nil {
				return fmt.Errorf("failed to publish message to %s: %w", subject, err)
			}

			return nil
		})
}

// PublishCore publishes data to subject with core NATS, not JetStream: for a
// message that no stream keeps, such as an enrollment request. The headers
// carry the trace context, as with Publish.
func (c *Client) PublishCore(
	ctx context.Context,
	subject string,
	data []byte,
) error {
	if c.NC == nil {
		return fmt.Errorf("NATS connection not established: call Connect() first")
	}

	c.logger.Debug(
		"publishing core message",
		slog.String("subject", subject),
		slog.Int("data_size", len(data)),
	)

	return c.publish(ctx, "nats.publish_core", subject, data,
		func(_ context.Context, msg *nats.Msg) error {
			if err := c.NC.PublishMsg(msg); err != nil {
				return fmt.Errorf("failed to publish to %s: %w", subject, err)
			}

			return nil
		})
}

// receive runs handle for a message on subject with header, inside a consumer
// span named name that continues the trace of the message from base.
func (c *Client) receive(
	base context.Context,
	name string,
	subject string,
	header nats.Header,
	handle func(context.Context) error,
) error {
	o := newOp(name, "", subjectAttr(subject))
	began := time.Now()
	ctx, span := c.tel.start(MessageContext(base, header), o, trace.SpanKindConsumer)
	err := handle(ctx)
	c.tel.end(ctx, o, span, began, err)

	return err
}

// Subscribe calls handler for each core NATS message on subject, until the
// subscription is drained or unsubscribed. ctx is the base of the context of
// each call.
func (c *Client) Subscribe(
	ctx context.Context,
	subject string,
	handler MsgHandler,
) (*nats.Subscription, error) {
	if c.NC == nil {
		return nil, fmt.Errorf("NATS connection not established: call Connect() first")
	}

	c.logger.Debug("subscribing to subject", slog.String("subject", subject))

	sub, err := c.NC.Subscribe(subject, func(msg *nats.Msg) {
		_ = c.receive(
			ctx,
			"nats.receive",
			msg.Subject,
			msg.Header,
			func(ctx context.Context) error {
				handler(ctx, msg)

				return nil
			},
		)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}

	return sub, nil
}

// Consume fetches the messages of the durable consumer on stream, and calls
// handler for each, until ctx ends. A message is acknowledged when handler
// returns nil. A handler that returns an error or panics leaves it
// unacknowledged, so that JetStream delivers it again.
func (c *Client) Consume(
	ctx context.Context,
	stream string,
	consumer string,
	handler JetStreamMessageHandler,
	opts *ConsumeOptions,
) error {
	if opts == nil {
		opts = &ConsumeOptions{
			MaxInFlight: 10, // Default limit
		}
	}

	c.logger.Debug(
		"starting message consumption",
		slog.String("stream", stream),
		slog.String("consumer", consumer),
		slog.String("queue_group", opts.QueueGroup),
		slog.Int("max_in_flight", opts.MaxInFlight),
	)

	cons, err := c.ExtJS.Consumer(ctx, stream, consumer)
	if err != nil {
		return fmt.Errorf(
			"failed to get consumer %s from stream %s: %w",
			consumer,
			stream,
			err,
		)
	}

	for {
		select {
		case <-ctx.Done():
			c.logger.Debug("stopping message consumption due to context cancellation")
			return ctx.Err()

		default:
			msgs, err := cons.Fetch(1)
			if err != nil {
				if err.Error() == "nats: timeout" {
					continue // Normal timeout, try again
				}
				c.logger.Error(
					"error fetching messages",
					slog.String("error", err.Error()),
				)
				continue
			}

			for msg := range msgs.Messages() {
				err := c.receive(ctx, "nats.process", msg.Subject(), msg.Headers(),
					func(ctx context.Context) error { return c.processMessage(ctx, msg, handler) })
				if err != nil {
					c.logger.Error(
						"error processing message",
						slog.String("error", err.Error()),
						slog.String("subject", msg.Subject()),
					)
					// Don't ack failed messages - they'll be redelivered
					continue
				}

				if err := msg.Ack(); err != nil {
					c.logger.Error(
						"error acknowledging message",
						slog.String("error", err.Error()),
					)
				}
			}
		}
	}
}

// processMessage runs handler for msg, and turns a panic into an error.
func (c *Client) processMessage(
	ctx context.Context,
	msg jetstream.Msg,
	handler JetStreamMessageHandler,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error(
				"panic in message handler",
				slog.Any("panic", r),
				slog.String("subject", msg.Subject()),
			)
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()

	return handler(ctx, msg)
}

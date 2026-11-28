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
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// MsgHandler handles a core NATS message. ctx carries the trace of the
// message, so that the work of the handler is in the trace of the request
// that published it.
type MsgHandler func(ctx context.Context, msg *nats.Msg)

// JetStreamMessageHandler handles a JetStream message, with ctx as for
// MsgHandler. An error makes Consume negatively acknowledge the message, so
// that JetStream delivers it again, after the BackOff of the consumer when it
// has one. An error that wraps ErrTerminate ends the message instead: JetStream
// does not deliver it again.
type JetStreamMessageHandler func(ctx context.Context, msg jetstream.Msg) error

// ErrTerminate is the error that a JetStreamMessageHandler wraps for a
// message that must not be delivered again, such as one that it cannot
// parse.
var ErrTerminate = errors.New("terminate the message")

// ConsumeOptions configures message consumption behavior.
type ConsumeOptions struct {
	// MaxInFlight is the number of messages that Consume handles at the same
	// time. Zero is DefaultMaxInFlight.
	MaxInFlight int
}

// DefaultMaxInFlight is the MaxInFlight of a Consume with no options or zero.
const DefaultMaxInFlight = 10

// fetchRetryFirst and fetchRetryLongest bound the wait of Consume after a fetch
// fails. The wait doubles after each failure in a row, and a fetch that
// succeeds sets it back to fetchRetryFirst.
const (
	fetchRetryFirst   = 100 * time.Millisecond
	fetchRetryLongest = 5 * time.Second
)

// terminalFetchError reports whether err is a failure of a fetch that a retry
// cannot fix: the connection is closed, or the consumer or its stream is
// gone. Consume returns it, so that its caller can make the consumer again.
func terminalFetchError(
	err error,
) bool {
	return errors.Is(err, nats.ErrConnectionClosed) ||
		errors.Is(err, jetstream.ErrConsumerNotFound) ||
		errors.Is(err, jetstream.ErrConsumerDeleted) ||
		errors.Is(err, jetstream.ErrStreamNotFound) ||
		errors.Is(err, jetstream.ErrNoStreamResponse)
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
	otel.GetTextMapPropagator().Inject(ctx, headerCarrier(header))
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

	return c.publish(ctx, "nats.publish", subject, data,
		func(ctx context.Context, msg *nats.Msg) error {
			// Inside send, so that a publish on a client that is not
			// connected has its span and its metric, as the other calls do.
			js, err := c.JetStream()
			if err != nil {
				return err
			}
			if _, err := js.PublishMsg(ctx, msg); err != nil {
				return fmt.Errorf("failed to publish message to %s: %w", subject, err)
			}

			return nil
		})
}

// PublishCore publishes data to subject with core NATS, not JetStream: for a
// message that no stream keeps, such as an enrollment request. The headers
// carry the trace context, as with Publish. A ctx that ended publishes
// nothing.
func (c *Client) PublishCore(
	ctx context.Context,
	subject string,
	data []byte,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}
	if err := c.withConn(func(NATSConnector) error { return nil }); err != nil {
		return err
	}

	c.logger.Debug(
		"publishing core message",
		slog.String("subject", subject),
		slog.Int("data_size", len(data)),
	)

	return c.publish(ctx, "nats.publish_core", subject, data,
		func(_ context.Context, msg *nats.Msg) error {
			if err := c.withConn(func(nc NATSConnector) error {
				return nc.PublishMsg(msg)
			}); err != nil {
				return fmt.Errorf("failed to publish to %s: %w", subject, err)
			}

			return nil
		})
}

// DefaultRequestTimeout bounds Request when its ctx has no deadline.
const DefaultRequestTimeout = 15 * time.Second

// Request sends data to subject with core NATS and returns the reply: for a
// request that a service answers, such as a join. The ctx deadline bounds
// the wait, or DefaultRequestTimeout when ctx has none. The headers carry
// the trace context, as with Publish.
func (c *Client) Request(
	ctx context.Context,
	subject string,
	data []byte,
) (*nats.Msg, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("failed to request %s: %w", subject, err)
	}
	// The wait for the reply runs outside the lock, so Close and Connect do
	// not wait for it. A Close during the wait closes the connection, and
	// the request ends with ErrConnectionClosed.
	var nc NATSConnector
	if err := c.withConn(func(conn NATSConnector) error {
		nc = conn

		return nil
	}); err != nil {
		return nil, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultRequestTimeout)
		defer cancel()
	}

	c.logger.Debug(
		"sending core request",
		slog.String("subject", subject),
		slog.Int("data_size", len(data)),
	)

	var reply *nats.Msg
	err := c.publish(ctx, "nats.request", subject, data,
		func(ctx context.Context, msg *nats.Msg) error {
			r, err := nc.RequestMsgWithContext(ctx, msg)
			if err != nil {
				return fmt.Errorf("failed to request %s: %w", subject, err)
			}
			reply = r

			return nil
		})
	if err != nil {
		return nil, err
	}

	return reply, nil
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

// Subscribe calls handler for each core NATS message on subject, until ctx
// ends or the subscription is drained or unsubscribed. ctx is the base of the
// context of each call, and the end of ctx unsubscribes. A ctx that ended
// subscribes nothing. Subscribe sets the closed handler of the subscription,
// so that an unsubscribe by hand stops the watch of ctx. A caller that sets
// its own closed handler replaces it, and the watch then lasts until ctx ends.
func (c *Client) Subscribe(
	ctx context.Context,
	subject string,
	handler MsgHandler,
) (*nats.Subscription, error) {
	c.logger.Debug("subscribing to subject", slog.String("subject", subject))
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}

	var sub *nats.Subscription
	err := c.withConn(func(nc NATSConnector) error {
		var subErr error
		sub, subErr = nc.Subscribe(subject, c.handlerOf(ctx, handler))
		return subErr
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}
	// A subscription that is already gone, such as after Close, needs no
	// unsubscribe. A subscription that closes first, such as one that the
	// caller unsubscribed, stops the AfterFunc, so that a long ctx keeps no
	// reference to it.
	stop := context.AfterFunc(ctx, func() {
		c.logger.Debug("unsubscribing at the end of ctx", slog.String("subject", subject))
		_ = sub.Unsubscribe()
	})
	sub.SetClosedHandler(func(string) {
		if stop() {
			c.logger.Debug(
				"subscription closed before the end of ctx",
				slog.String("subject", subject),
			)
		}
	})

	return sub, nil
}

// handlerOf is the core NATS handler that runs handler for each message, in
// a consumer span, with a panic in handler logged and kept from the process.
func (c *Client) handlerOf(
	ctx context.Context,
	handler MsgHandler,
) nats.MsgHandler {
	return func(msg *nats.Msg) {
		_ = c.receive(
			ctx,
			"nats.receive",
			msg.Subject,
			msg.Header,
			func(ctx context.Context) (err error) {
				defer func() {
					if r := recover(); r != nil {
						c.logger.Error(
							"panic in message handler",
							slog.Any("panic", r),
							slog.String("subject", msg.Subject),
						)
						err = fmt.Errorf("handler panicked: %v", r)
					}
				}()
				handler(ctx, msg)

				return nil
			},
		)
	}
}

// Consume fetches the messages of the durable consumer on stream, and calls
// handler for each, until ctx ends. It handles up to MaxInFlight messages at
// the same time, and fetches only as many as it has room for. A wait for a
// message also ends when ctx ends. Consume returns only after each handler
// that it started has returned.
//
// A message is acknowledged when handler returns nil. A handler that returns
// an error or panics makes it negatively acknowledged, so that JetStream
// delivers it again. An error that wraps ErrTerminate ends it.
//
// A fetch that fails is tried again after a wait that doubles, up to
// fetchRetryLongest. A failure that a retry cannot fix, such as a closed
// connection or a consumer that is gone, ends Consume with that error.
func (c *Client) Consume(
	ctx context.Context,
	stream string,
	consumer string,
	handler JetStreamMessageHandler,
	opts *ConsumeOptions,
) error {
	limit := DefaultMaxInFlight
	if opts != nil && opts.MaxInFlight > 0 {
		limit = opts.MaxInFlight
	}

	c.logger.Debug(
		"starting message consumption",
		slog.String("stream", stream),
		slog.String("consumer", consumer),
		slog.Int("max_in_flight", limit),
	)

	js, err := c.JetStream()
	if err != nil {
		return err
	}
	cons, err := js.Consumer(ctx, stream, consumer)
	if err != nil {
		return fmt.Errorf(
			"failed to get consumer %s from stream %s: %w",
			consumer,
			stream,
			err,
		)
	}

	slots := make(chan struct{}, limit)
	var running sync.WaitGroup
	defer running.Wait()

	wait := fetchRetryFirst
	for {
		// Take one slot, waiting for a handler to finish when all are in
		// use, then each other slot that is free. A select picks at random
		// when both are ready, so an ended ctx is checked first.
		if ctx.Err() != nil {
			c.logger.Debug("stopping message consumption due to context cancellation")
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			c.logger.Debug("stopping message consumption due to context cancellation")
			return ctx.Err()
		case slots <- struct{}{}:
		}
		n := 1
	more:
		for n < limit {
			select {
			case slots <- struct{}{}:
				n++
			default:
				break more
			}
		}

		got, err := c.fetch(ctx, cons, n, handler, slots, &running)
		for range n - got {
			<-slots
		}
		switch {
		case err == nil:
			wait = fetchRetryFirst
		case ctx.Err() != nil:
			return ctx.Err()
		case terminalFetchError(err):
			return fmt.Errorf("failed to fetch from consumer %s: %w", consumer, err)
		default:
			c.logger.Error("error fetching messages", slog.String("error", err.Error()))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			wait = min(2*wait, fetchRetryLongest)
		}
	}
}

// fetch fetches up to n messages from cons, and starts a handler for each in
// running. Each handler gives back its slot when it returns. fetch returns the
// number of messages that it started, and the error of the fetch or of the
// batch. A fetch that ends with no message is not an error.
func (c *Client) fetch(
	ctx context.Context,
	cons jetstream.Consumer,
	n int,
	handler JetStreamMessageHandler,
	slots chan struct{},
	running *sync.WaitGroup,
) (int, error) {
	// FetchContext ends the wait for a message when ctx ends, so a shutdown
	// does not wait for the expiry of the pull request.
	batch, err := cons.Fetch(n, jetstream.FetchContext(ctx))
	if err != nil {
		if errors.Is(err, nats.ErrTimeout) {
			return 0, nil
		}

		return 0, err
	}

	got := 0
	for msg := range batch.Messages() {
		got++
		running.Add(1)
		go func() {
			defer running.Done()
			defer func() { <-slots }()
			c.handle(ctx, msg, handler)
		}()
	}
	if err := batch.Error(); err != nil && !errors.Is(err, nats.ErrTimeout) {
		return got, err
	}

	return got, nil
}

// handle runs handler for msg in its span, and acknowledges, negatively
// acknowledges or ends msg by the result.
func (c *Client) handle(
	ctx context.Context,
	msg jetstream.Msg,
	handler JetStreamMessageHandler,
) {
	err := c.receive(ctx, "nats.process", msg.Subject(), msg.Headers(),
		func(ctx context.Context) error { return c.processMessage(ctx, msg, handler) })

	reply, verb := msg.Ack, "acknowledging"
	switch {
	case errors.Is(err, ErrTerminate):
		reply, verb = msg.Term, "terminating"
	case err != nil:
		reply, verb = msg.Nak, "negatively acknowledging"
	}
	if err != nil {
		c.logger.Error(
			"error processing message",
			slog.String("error", err.Error()),
			slog.String("subject", msg.Subject()),
		)
	}
	if err := reply(); err != nil {
		c.logger.Error(
			"error "+verb+" message",
			slog.String("error", err.Error()),
		)
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

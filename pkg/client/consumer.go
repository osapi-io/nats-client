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

	"github.com/nats-io/nats.go/jetstream"
)

// JetStreamMessageHandler defines the signature for JetStream message handling functions.
// defaultMaxInFlight caps unacknowledged messages when the caller passes no
// ConsumeOptions.
const defaultMaxInFlight = 10

// fetchTimeoutMessage is what a Fetch returns when no message arrived before
// the deadline. It is the idle case, not a failure.
const fetchTimeoutMessage = "nats: timeout"

type JetStreamMessageHandler func(msg jetstream.Msg) error

// ConsumeOptions configures message consumption behavior.
type ConsumeOptions struct {
	// QueueGroup for load balancing across multiple consumers (optional)
	QueueGroup string
	// MaxInFlight limits the number of unacknowledged messages
	MaxInFlight int
}

// ConsumeMessages subscribes to a JetStream consumer and processes messages with the provided handler.
// This provides a clean abstraction for message consumption with proper context handling.
func (c *Client) ConsumeMessages(
	ctx context.Context,
	streamName string,
	consumerName string,
	handler JetStreamMessageHandler,
	opts *ConsumeOptions,
) error {
	if opts == nil {
		opts = &ConsumeOptions{
			MaxInFlight: defaultMaxInFlight,
		}
	}

	c.logger.Debug(
		"starting message consumption",
		slog.String("stream", streamName),
		slog.String("consumer", consumerName),
		slog.String("queue_group", opts.QueueGroup),
		slog.Int("max_in_flight", opts.MaxInFlight),
	)

	// Get the consumer
	consumer, err := c.ExtJS.Consumer(ctx, streamName, consumerName)
	if err != nil {
		return fmt.Errorf(
			"failed to get consumer %s from stream %s: %w",
			consumerName,
			streamName,
			err,
		)
	}

	// Start consuming messages
	for {
		select {
		case <-ctx.Done():
			c.logger.Debug(
				"stopping message consumption due to context cancellation",
			)

			return ctx.Err()

		default:
			c.fetchAndDeliver(consumer, handler)
		}
	}
}

// fetchAndDeliver fetches one batch and hands each message to the handler.
//
// Nothing here returns an error. A fetch that times out is the normal idle
// case, and a fetch that fails for another reason is logged and retried on the
// next pass — consumption is a loop that keeps running, not an operation that
// succeeds once.
func (c *Client) fetchAndDeliver(
	consumer jetstream.Consumer,
	handler JetStreamMessageHandler,
) {
	msgs, err := consumer.Fetch(1)
	if err != nil {
		if err.Error() == fetchTimeoutMessage {
			return
		}

		c.logger.Error(
			"error fetching messages",
			slog.String("error", err.Error()),
		)

		return
	}

	for msg := range msgs.Messages() {
		c.deliver(msg, handler)
	}
}

// deliver runs the handler against one message and acknowledges it.
//
// A message the handler failed on is deliberately left unacknowledged, so
// JetStream redelivers it.
func (c *Client) deliver(
	msg jetstream.Msg,
	handler JetStreamMessageHandler,
) {
	if err := c.processMessage(msg, handler); err != nil {
		c.logger.Error(
			"error processing message",
			slog.String("error", err.Error()),
			slog.String("subject", msg.Subject()),
		)

		return
	}

	if err := msg.Ack(); err != nil {
		c.logger.Error(
			"error acknowledging message",
			slog.String("error", err.Error()),
		)
	}
}

// processMessage handles individual message processing with proper error handling.
func (c *Client) processMessage(
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

	return handler(msg)
}

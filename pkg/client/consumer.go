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
			MaxInFlight: 10, // Default limit
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
			c.logger.Debug("stopping message consumption due to context cancellation")
			return ctx.Err()

		default:
			// Fetch messages with timeout
			msgs, err := consumer.Fetch(1)
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

			// Process each message
			for msg := range msgs.Messages() {
				if err := c.processMessage(msg, handler); err != nil {
					c.logger.Error(
						"error processing message",
						slog.String("error", err.Error()),
						slog.String("subject", msg.Subject()),
					)
					// Don't ack failed messages - they'll be redelivered
					continue
				}

				// Acknowledge successful processing
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

// Copyright (c) 2026 John Dewey

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
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

// Subscribe creates a core NATS subscription on the given subject.
// Messages are delivered to the handler callback. This uses plain NATS
// pub/sub (not JetStream) — suitable for fire-and-forget messaging
// like enrollment requests and key rotation notifications.
func (c *Client) Subscribe(
	subject string,
	handler nats.MsgHandler,
) (*nats.Subscription, error) {
	if c.NC == nil {
		return nil, fmt.Errorf("NATS connection not established: call Connect() first")
	}

	c.logger.Debug(
		"subscribing to subject",
		slog.String("subject", subject),
	)

	sub, err := c.NC.Subscribe(subject, handler)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}

	return sub, nil
}

// PublishCore publishes a message using core NATS (not JetStream).
// Unlike Publish, this does not require JetStream stream routing and
// does not inject OpenTelemetry trace headers. Use this for simple
// fire-and-forget messaging like enrollment responses and key rotation.
func (c *Client) PublishCore(
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

	if err := c.NC.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}

	return nil
}

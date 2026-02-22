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
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// CreateOrUpdateStreamWithConfig creates or updates a JetStream stream with the provided configuration.
func (c *Client) CreateOrUpdateStreamWithConfig(
	ctx context.Context,
	streamConfig jetstream.StreamConfig,
) error {
	c.logger.Debug(
		"creating stream",
		slog.String("name", streamConfig.Name),
		slog.String("subjects", strings.Join(streamConfig.Subjects, ", ")),
	)

	_, err := c.ExtJS.CreateOrUpdateStream(ctx, streamConfig)
	if err != nil {
		return fmt.Errorf("error creating/updating stream %s: %w", streamConfig.Name, err)
	}

	return nil
}

// CreateOrUpdateConsumerWithConfig creates or updates a JetStream consumer with the provided configuration.
func (c *Client) CreateOrUpdateConsumerWithConfig(
	ctx context.Context,
	streamName string,
	consumerConfig jetstream.ConsumerConfig,
) error {
	c.logger.Debug(
		"creating consumer",
		slog.String("durable", consumerConfig.Durable),
		slog.String("stream", streamName),
	)

	_, err := c.ExtJS.CreateOrUpdateConsumer(ctx, streamName, consumerConfig)
	if err != nil {
		return fmt.Errorf("error creating consumer for stream %s: %w", streamName, err)
	}

	return nil
}

// CreateOrUpdateJetStreamWithConfig configures a JetStream stream and its consumers.
// This is a convenience method that creates both stream and consumers in one call.
func (c *Client) CreateOrUpdateJetStreamWithConfig(
	ctx context.Context,
	streamConfig jetstream.StreamConfig,
	consumerConfigs ...jetstream.ConsumerConfig,
) error {
	// Create or update the stream
	if err := c.CreateOrUpdateStreamWithConfig(ctx, streamConfig); err != nil {
		return err
	}

	// Create or update each consumer
	for _, consumerConfig := range consumerConfigs {
		if err := c.CreateOrUpdateConsumerWithConfig(ctx, streamConfig.Name, consumerConfig); err != nil {
			return err
		}
	}

	c.logger.Debug("jet stream setup completed successfully")

	return nil
}

// GetStreamInfo retrieves information about a JetStream stream.
func (c *Client) GetStreamInfo(
	ctx context.Context,
	streamName string,
) (*jetstream.StreamInfo, error) {
	stream, err := c.ExtJS.Stream(ctx, streamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream %s: %w", streamName, err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info for %s: %w", streamName, err)
	}

	return info, nil
}

// Publish publishes a message to a JetStream subject.
// If the context carries an OpenTelemetry span, the trace context is
// automatically propagated via NATS message headers.
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
		return fmt.Errorf("JetStream not initialized: call Connect() first")
	}

	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(http.Header(msg.Header)))

	_, err := c.ExtJS.PublishMsg(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to publish message to %s: %w", subject, err)
	}

	return nil
}

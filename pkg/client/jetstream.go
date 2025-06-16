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
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// CreateOrUpdateStreamWithConfig creates or updates a JetStream stream with the provided configuration.
func (c *Client) CreateOrUpdateStreamWithConfig(
	ctx context.Context,
	streamConfig *nats.StreamConfig,
) error {
	c.logger.Info(
		"creating stream",
		slog.String("name", streamConfig.Name),
		slog.String("subjects", strings.Join(streamConfig.Subjects, ", ")),
	)

	return c.createOrUpdateStream(streamConfig, streamConfig.Name)
}

// CreateOrUpdateConsumerWithConfig creates or updates a JetStream consumer with the provided configuration.
func (c *Client) CreateOrUpdateConsumerWithConfig(
	ctx context.Context,
	streamName string,
	consumerConfig jetstream.ConsumerConfig,
) error {
	c.logger.Info(
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
	streamConfig *nats.StreamConfig,
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

	c.logger.Info("jet stream setup completed successfully")

	return nil
}

// createOrUpdateStream provisions (or creates) a stream using the native JetStream API.
func (c *Client) createOrUpdateStream(
	streamConfig *nats.StreamConfig,
	streamName string,
) error {
	_, err := c.NativeJS.AddStream(streamConfig)
	if err != nil {
		// Check if the error indicates that the stream already exists.
		if strings.Contains(err.Error(), "already in use") {
			c.logger.Info(
				"stream already exists; updating stream",
				slog.String("stream", streamName),
			)

			_, err = c.NativeJS.UpdateStream(streamConfig)
			if err != nil {
				return fmt.Errorf("error updating stream %s: %w", streamName, err)
			}
		} else {
			return fmt.Errorf("error creating stream %s: %w", streamName, err)
		}
	}

	return nil
}

// GetStreamInfo retrieves information about a JetStream stream.
func (c *Client) GetStreamInfo(
	ctx context.Context,
	streamName string,
) (*nats.StreamInfo, error) {
	// Use the native JetStream API to get stream info
	info, err := c.NativeJS.StreamInfo(streamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info for %s: %w", streamName, err)
	}

	return info, nil
}

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
)

// CreateOrUpdateJetStream configures JetStream streams and consumers.
func (c *Client) CreateOrUpdateJetStream(
	ctx context.Context,
	streamConfigs ...*StreamConfig,
) error {
	if len(streamConfigs) == 0 {
		return fmt.Errorf("jetstream is enabled but no stream configuration was provided")
	}

	// For each stream, we provision the stream and then iterate over its consumer
	// configurations to create or update each consumer.
	for _, stream := range streamConfigs {
		streamConfig := stream.StreamConfig
		c.logger.Info(
			"creating stream",
			slog.String("name", stream.Name),
			slog.String("subjects", strings.Join(stream.Subjects, ", ")),
		)

		// Use the helper to provision the stream.
		if err := c.createOrUpdateStream(streamConfig, stream.Name); err != nil {
			return err
		}

		// For each consumer configuration, use the extended API to create or update the consumer.
		for _, consumer := range stream.Consumers {
			jsConsumerConfig := *consumer.ConsumerConfig

			c.logger.Info(
				"creating consumer",
				slog.String("durable", consumer.Durable),
				slog.String("stream", stream.Name),
			)

			// Use the extended JetStream API to create or update the consumer.
			_, err := c.ExtJS.CreateOrUpdateConsumer(ctx, stream.Name, jsConsumerConfig)
			if err != nil {
				return fmt.Errorf("error creating consumer for stream %s: %w", stream.Name, err)
			}
		}
	}

	c.logger.Info("jet stream setup completed successfully")

	return nil
}

// createOrUpdateStream provisions (or creates) a stream using the native JetStream API.
func (c *Client) createOrUpdateStream(streamConfig *nats.StreamConfig, streamName string) error {
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

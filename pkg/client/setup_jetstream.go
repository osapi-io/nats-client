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
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

// NewJetStreamContext creates a NATS connection with the given authentication options.
func NewJetStreamContext(opts *ClientOptions) (nats.JetStreamContext, error) {
	natsURL := fmt.Sprintf("nats://%s:%d", opts.Host, opts.Port)
	var nc *nats.Conn
	var err error

	switch opts.Auth.AuthType {
	case NoAuth:
		nc, err = nats.Connect(natsURL)
	case UserPassAuth:
		nc, err = nats.Connect(natsURL, nats.UserInfo(opts.Auth.Username, opts.Auth.Password))
	case NKeyAuth:
		seed, readErr := os.ReadFile(opts.Auth.NKeyFile)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read nkey seed file: %w", readErr)
		}

		kp, kpErr := nkeys.FromSeed(seed)
		if kpErr != nil {
			return nil, fmt.Errorf("failed to parse nkey seed: %w", kpErr)
		}

		pubKey, pubErr := kp.PublicKey()
		if pubErr != nil {
			return nil, fmt.Errorf("failed to get public key from nkey: %w", pubErr)
		}

		nc, err = nats.Connect(natsURL, nats.Nkey(pubKey, func(nonce []byte) ([]byte, error) {
			return kp.Sign(nonce)
		}))
	default:
		return nil, fmt.Errorf("unsupported authentication method")
	}

	if err != nil {
		return nil, fmt.Errorf("error connecting to nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("error enabling JetStream: %w", err)
	}

	return js, nil
}

// SetupJetStream configures JetStream streams and consumers.
func (c *Client) SetupJetStream(js nats.JetStreamContext, streamConfigs ...*StreamConfig) error {
	if len(streamConfigs) == 0 {
		return fmt.Errorf("jetstream is enabled but no stream configuration was provided")
	}

	for _, stream := range streamConfigs {
		natsStreamConfig := stream.StreamConfig
		c.logger.Debug(
			"creating stream",
			slog.String("name", stream.Name),
			slog.String("subjects", strings.Join(stream.Subjects, ", ")),
		)

		_, err := js.AddStream(natsStreamConfig)
		if err != nil {
			return fmt.Errorf("error creating stream %s: %w", stream.Name, err)
		}

		// Iterate over each consumer tied to the stream
		for _, consumer := range stream.Consumers {
			natsConsumerConfig := consumer.ConsumerConfig
			c.logger.Debug(
				"creating consumer",
				slog.String("durable", consumer.Durable),
				slog.String("stream", stream.Name),
			)

			_, err := js.AddConsumer(stream.Name, natsConsumerConfig)
			if err != nil {
				return fmt.Errorf("error creating consumer for stream %s: %w", stream.Name, err)
			}
		}
	}

	c.logger.Info("jet stream setup completed successfully")

	return nil
}

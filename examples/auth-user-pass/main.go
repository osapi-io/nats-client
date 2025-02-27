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

package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/osapi-io/nats-client/pkg/client"
)

func main() {
	logger := slog.Default()

	opts := &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{
			AuthType: client.UserPassAuth,
			Username: "myuser",
			Password: "mypassword",
		},
	}

	c := client.New(logger, opts)

	if err := c.Connect(); err != nil {
		logger.Error("failed to connect", "error", err)
		os.Exit(1)
	}
	defer c.NC.Close()
	logger.Info("connected", "url", c.NC.ConnectedUrl())

	streamOpts := &client.StreamConfig{
		StreamConfig: &nats.StreamConfig{
			Name:     "STREAM2",
			Subjects: []string{"stream2.*"},
			Storage:  nats.FileStorage,
			Replicas: 1,
		},
		Consumers: []*client.ConsumerConfig{
			{
				ConsumerConfig: &jetstream.ConsumerConfig{
					Durable:    "consumer3",
					AckPolicy:  jetstream.AckExplicitPolicy,
					MaxDeliver: 5,
					AckWait:    30 * time.Second,
				},
			},
			{
				ConsumerConfig: &jetstream.ConsumerConfig{
					Durable:    "consumer4",
					AckPolicy:  jetstream.AckExplicitPolicy,
					MaxDeliver: 5,
					AckWait:    30 * time.Second,
				},
			},
		},
	}

	ctx := context.Background()

	if err := c.SetupJetStream(ctx, streamOpts); err != nil {
		logger.Error("failed setting up jetstream", "error", err)
		os.Exit(1)
	}
}

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
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/osapi-io/nats-client/pkg/client"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: worker <priority> <id>")
		os.Exit(1)
	}

	priority := os.Args[1]
	id := os.Args[2]
	consumerName := fmt.Sprintf("worker_%s", priority)
	streamName := "jobs"
	workerName := fmt.Sprintf("worker_%s_%s", priority, id)
	logger := slog.Default()

	opts := &client.Options{
		Host: "localhost",
		Port: 4222,
		Name: workerName,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	}

	c := client.New(logger, opts)

	if err := c.Connect(); err != nil {
		logger.Error("failed to connect", "error", err)
		os.Exit(1)
	}
	defer c.NC.Close()
	logger.Info("connected", "url", c.NC.ConnectedUrl())

	// Provision the stream and consumer using the native API.
	// (For example, this creates a stream "jobs" with a consumer named worker_high
	// that only receives messages matching "jobs.<priority.>" such as "jobs.high.>")
	streamOpts := &client.StreamConfig{
		StreamConfig: &nats.StreamConfig{
			Name:              streamName,
			Subjects:          []string{"jobs.*.*"},
			Storage:           nats.FileStorage,
			Replicas:          1,
			Retention:         nats.WorkQueuePolicy,
			Discard:           nats.DiscardNew,
			MaxMsgs:           20000,
			MaxMsgsPerSubject: -1,
			MaxBytes:          256 * 1024 * 1024,  // 256 MiB
			MaxAge:            7 * 24 * time.Hour, // 7 days
			Duplicates:        2 * time.Minute,
			DenyDelete:        false,
			DenyPurge:         false,
			AllowRollup:       false,
			AllowDirect:       true,
		},
		Consumers: []*client.ConsumerConfig{
			{
				ConsumerConfig: &jetstream.ConsumerConfig{
					Durable:       consumerName,
					AckPolicy:     jetstream.AckExplicitPolicy,
					MaxDeliver:    4,
					AckWait:       30 * time.Second,
					FilterSubject: fmt.Sprintf("jobs.%s.>", priority),
				},
			},
		},
	}

	// Provision the DLQ stream to capture advisories for consumer max deliveries.
	// When a consumer reaches its max delivery attempts, NATS publishes an advisory on:
	//   $JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.jobs.{consumer_name}
	dlqStreamOpts := &client.StreamConfig{
		StreamConfig: &nats.StreamConfig{
			Name:     "jobs_dlq",
			Subjects: []string{"$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.jobs.*"},
			Storage:  nats.FileStorage,
			Replicas: 1,
			Metadata: map[string]string{
				"dead_letter_queue": "true",
			},
		},
	}

	ctx := context.Background()

	if err := c.CreateOrUpdateJetStream(ctx, streamOpts, dlqStreamOpts); err != nil {
		logger.Error("failed setting up jetstream", "error", err)
		os.Exit(1)
	}

	// Later, retrieve the stream and consumer using the extended API.
	// This high-level API call lets you access operations like Consume.
	stream, err := c.ExtJS.Stream(ctx, streamName)
	if err != nil {
		logger.Error("error retrieving stream", "error", err)
		os.Exit(1)
	}

	// Retrieve the consumer object for the specified consumer name from the stream.
	// This high-level API call returns a Consumer instance, which can then be used
	// to subscribe and consume messages (e.g., via its Consume method).
	cons, err := stream.Consumer(ctx, consumerName)
	if err != nil {
		logger.Error("error retrieving consumer", "error", err)
		os.Exit(1)
	}

	// Use the high-level consumer API to start consuming messages.
	sub, err := cons.Consume(func(msg jetstream.Msg) {
		meta, err := msg.Metadata()
		if err != nil {
			logger.Info("error getting metadata", "error", err)
			return
		}
		// Example: if priority is "low", simulate an error.
		if priority == "low" {
			logger.Error("error processing job")
			return
		}
		logger.Info("received message", "sequence", meta.Sequence.Stream)
		time.Sleep(10 * time.Millisecond)
		msg.Ack()
	})
	if err != nil {
		logger.Error("failed to start consumer", "error", err)
		os.Exit(1)
	}

	// Wait for a termination signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// Stop the consumer subscription gracefully.
	sub.Stop()
	logger.Info("worker stopped gracefully")
}

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
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// CreateKVBucket ensures a KV bucket exists and returns the KeyValue interface.
func (c *Client) CreateKVBucket(
	bucketName string,
) (nats.KeyValue, error) {
	c.logger.Info(
		"creating KV bucket",
		slog.String("bucket", bucketName),
	)

	kv, err := c.NativeJS.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: bucketName,
	})
	if err != nil {
		return nil, err
	}

	return kv, nil
}

// CreateKVBucketWithConfig ensures a KV bucket exists with the provided configuration and returns the KeyValue interface.
func (c *Client) CreateKVBucketWithConfig(
	config *nats.KeyValueConfig,
) (nats.KeyValue, error) {
	c.logger.Info(
		"creating KV bucket with config",
		slog.String("bucket", config.Bucket),
		slog.String("description", config.Description),
	)

	kv, err := c.NativeJS.CreateKeyValue(config)
	if err != nil {
		return nil, err
	}

	return kv, nil
}

// RequestReplyOptions configures request/reply behavior.
type RequestReplyOptions struct {
	// RequestID to use (default: generated UUID)
	RequestID string
	// Timeout for waiting for response (default: 30s)
	Timeout time.Duration
	// PollInterval for checking KV store (default: 100ms)
	PollInterval time.Duration
}

// PublishAndWaitKV publishes a message and waits for a response in a KV bucket.
// This implements an async request/reply pattern using KV for response storage.
func (c *Client) PublishAndWaitKV(
	ctx context.Context,
	subject string,
	data []byte,
	kvBucket nats.KeyValue,
	opts *RequestReplyOptions,
) ([]byte, error) {
	if opts == nil {
		opts = &RequestReplyOptions{}
	}

	// Set defaults
	if opts.RequestID == "" {
		opts.RequestID = uuid.New().String()
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = 100 * time.Millisecond
	}

	// Add request ID to headers
	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	msg.Header.Set("Request-ID", opts.RequestID)

	c.logger.Info(
		"publishing request with KV reply",
		slog.String("request_id", opts.RequestID),
		slog.String("subject", subject),
	)

	// Publish via JetStream
	if _, err := c.ExtJS.PublishMsg(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to publish: %w", err)
	}

	// Wait for response with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	return c.waitForKVResponse(timeoutCtx, kvBucket, opts.RequestID, opts.PollInterval)
}

// waitForKVResponse polls a KV bucket for a response.
func (c *Client) waitForKVResponse(
	ctx context.Context,
	kv nats.KeyValue,
	key string,
	pollInterval time.Duration,
) ([]byte, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for response: %w", ctx.Err())
		case <-ticker.C:
			entry, err := kv.Get(key)
			if err != nil {
				if err == nats.ErrKeyNotFound {
					continue // Not ready yet
				}
				return nil, fmt.Errorf("failed to get response: %w", err)
			}

			c.logger.Info(
				"received KV response",
				slog.String("key", key),
				slog.Uint64("revision", entry.Revision()),
			)

			// Optionally delete the response after reading
			// _ = kv.Delete(ctx, key)

			return entry.Value(), nil
		}
	}
}

// WatchKV watches a KV bucket for responses matching a pattern.
// This is useful for collecting responses from multiple workers.
//
// Note: Test coverage for this function is intentionally limited to error paths
// and setup logic. The goroutine's event forwarding logic is not covered due to
// the complexity of testing async behavior without introducing flaky tests.
func (c *Client) WatchKV(
	ctx context.Context,
	kv nats.KeyValue,
	pattern string,
) (<-chan nats.KeyValueEntry, error) {
	c.logger.Info(
		"creating KV watcher",
		slog.String("pattern", pattern),
	)

	watcher, err := kv.Watch(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	out := make(chan nats.KeyValueEntry)

	go func() {
		defer close(out)
		defer func() {
			_ = watcher.Stop()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case entry := <-watcher.Updates():
				if entry != nil {
					select {
					case out <- entry:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return out, nil
}

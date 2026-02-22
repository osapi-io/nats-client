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
	"github.com/nats-io/nats.go/jetstream"
)

// CreateOrUpdateKVBucket creates or updates a KV bucket using the jetstream API,
// which natively supports upsert semantics. Returns the jetstream.KeyValue interface.
func (c *Client) CreateOrUpdateKVBucket(
	ctx context.Context,
	bucketName string,
) (jetstream.KeyValue, error) {
	return c.CreateOrUpdateKVBucketWithConfig(ctx, jetstream.KeyValueConfig{
		Bucket: bucketName,
	})
}

// CreateOrUpdateKVBucketWithConfig creates or updates a KV bucket with the provided
// configuration using the jetstream API. This uses native upsert semantics and
// does not require a fallback hack for existing buckets.
func (c *Client) CreateOrUpdateKVBucketWithConfig(
	ctx context.Context,
	config jetstream.KeyValueConfig,
) (jetstream.KeyValue, error) {
	c.logger.Debug(
		"creating/updating KV bucket",
		slog.String("bucket", config.Bucket),
	)

	kv, err := c.ExtJS.CreateOrUpdateKeyValue(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update KV bucket %s: %w", config.Bucket, err)
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
	kvBucket jetstream.KeyValue,
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

	c.logger.Debug(
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
	kv jetstream.KeyValue,
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
			entry, err := kv.Get(ctx, key)
			if err != nil {
				if err == jetstream.ErrKeyNotFound {
					continue // Not ready yet
				}
				return nil, fmt.Errorf("failed to get response: %w", err)
			}

			c.logger.Debug(
				"received KV response",
				slog.String("key", key),
				slog.Uint64("revision", entry.Revision()),
			)

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
	kv jetstream.KeyValue,
	pattern string,
) (<-chan jetstream.KeyValueEntry, error) {
	c.logger.Debug(
		"creating KV watcher",
		slog.String("pattern", pattern),
	)

	watcher, err := kv.Watch(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	out := make(chan jetstream.KeyValueEntry)

	go func() {
		defer close(out)
		defer func() {
			_ = watcher.Stop()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case entry, ok := <-watcher.Updates():
				if !ok {
					// Channel is closed, exit the goroutine
					return
				}
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

// KVPut stores a value in the specified KV bucket.
func (c *Client) KVPut(
	bucket string,
	key string,
	value []byte,
) error {
	kv, err := c.CreateOrUpdateKVBucket(context.Background(), bucket)
	if err != nil {
		return fmt.Errorf("failed to get KV bucket %s: %w", bucket, err)
	}

	_, err = kv.Put(context.Background(), key, value)
	if err != nil {
		return fmt.Errorf("failed to put key %s in bucket %s: %w", key, bucket, err)
	}

	return nil
}

// KVGet retrieves a value from the specified KV bucket.
func (c *Client) KVGet(
	bucket string,
	key string,
) ([]byte, error) {
	kv, err := c.CreateOrUpdateKVBucket(context.Background(), bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket %s: %w", bucket, err)
	}

	entry, err := kv.Get(context.Background(), key)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s from bucket %s: %w", key, bucket, err)
	}

	return entry.Value(), nil
}

// KVDelete removes a key from the specified KV bucket.
func (c *Client) KVDelete(
	bucket string,
	key string,
) error {
	kv, err := c.CreateOrUpdateKVBucket(context.Background(), bucket)
	if err != nil {
		return fmt.Errorf("failed to get KV bucket %s: %w", bucket, err)
	}

	err = kv.Delete(context.Background(), key)
	if err != nil {
		return fmt.Errorf("failed to delete key %s from bucket %s: %w", key, bucket, err)
	}

	return nil
}

// KVKeys returns all keys from the specified KV bucket.
func (c *Client) KVKeys(
	bucket string,
) ([]string, error) {
	kv, err := c.CreateOrUpdateKVBucket(context.Background(), bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket %s: %w", bucket, err)
	}

	keys, err := kv.ListKeys(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get keys from bucket %s: %w", bucket, err)
	}

	var result []string
	for key := range keys.Keys() {
		result = append(result, key)
	}

	return result, nil
}

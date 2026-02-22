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
)

// KVPutAndPublish implements the pattern of storing data in KV and sending a notification via stream.
// This is useful for workflow systems where you want persistent storage + event notification.
func (c *Client) KVPutAndPublish(
	ctx context.Context,
	kvBucket string,
	key string,
	data []byte,
	notifySubject string,
) (uint64, error) {
	c.logger.Debug(
		"storing item and sending notification",
		slog.String("kv_bucket", kvBucket),
		slog.String("key", key),
		slog.String("subject", notifySubject),
	)

	// Get the KV bucket
	kv, err := c.CreateOrUpdateKVBucket(ctx, kvBucket)
	if err != nil {
		return 0, fmt.Errorf("failed to get KV bucket '%s': %w", kvBucket, err)
	}

	// Store in KV
	revision, err := kv.Put(ctx, key, data)
	if err != nil {
		return 0, fmt.Errorf("failed to store data in KV: %w", err)
	}

	// Send notification with the key
	_, err = c.ExtJS.Publish(ctx, notifySubject, []byte(key))
	if err != nil {
		return 0, fmt.Errorf("failed to send notification: %w", err)
	}

	c.logger.Debug(
		"item stored and notification sent",
		slog.String("key", key),
		slog.Uint64("revision", revision),
	)

	return revision, nil
}

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
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

// CreateOrUpdateObjectStore creates or updates a NATS Object Store bucket
// with the provided configuration.
//
// NATS does not allow changing the storage type on an existing stream. If
// CreateOrUpdateObjectStore fails with a "can not change storage type"
// error, this method retries without the storage field.
func (c *Client) CreateOrUpdateObjectStore(
	ctx context.Context,
	cfg jetstream.ObjectStoreConfig,
) (jetstream.ObjectStore, error) {
	c.logger.Debug(
		"creating/updating Object Store bucket",
		slog.String("bucket", cfg.Bucket),
	)

	os, err := c.ExtJS.CreateOrUpdateObjectStore(ctx, cfg)
	if err == nil {
		return os, nil
	}

	if strings.Contains(err.Error(), "can not change storage type") {
		c.logger.Debug(
			"retrying Object Store update without storage type",
			slog.String("bucket", cfg.Bucket),
		)

		cfg.Storage = 0
		os, retryErr := c.ExtJS.CreateOrUpdateObjectStore(ctx, cfg)
		if retryErr != nil {
			return nil, fmt.Errorf(
				"failed to create/update Object Store bucket %s: %w",
				cfg.Bucket,
				retryErr,
			)
		}

		return os, nil
	}

	return nil, fmt.Errorf(
		"failed to create/update Object Store bucket %s: %w",
		cfg.Bucket,
		err,
	)
}

// ObjectStore returns an existing NATS Object Store handle by name.
func (c *Client) ObjectStore(
	ctx context.Context,
	name string,
) (jetstream.ObjectStore, error) {
	c.logger.Debug(
		"getting Object Store bucket",
		slog.String("bucket", name),
	)

	os, err := c.ExtJS.ObjectStore(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get Object Store bucket %s: %w", name, err)
	}

	return os, nil
}

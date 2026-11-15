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
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
)

// objectAttr is the attribute of the name of an object, on its span only.
func objectAttr(
	name string,
) attribute.KeyValue {
	return attribute.String("nats.object.name", name)
}

// ObjectStoreCreateOrUpdate creates the Object Store bucket of config, or
// updates it to match. As with KVCreateOrUpdate, a bucket whose storage type
// differs stays as it is.
func (c *Client) ObjectStoreCreateOrUpdate(
	ctx context.Context,
	config jetstream.ObjectStoreConfig,
) error {
	return observeErr(ctx, c.tel, newOp("objectstore.create_or_update", config.Bucket),
		func(ctx context.Context) error {
			c.logger.Debug(
				"creating/updating Object Store bucket",
				slog.String("bucket", config.Bucket),
			)

			store, err := c.ExtJS.CreateOrUpdateObjectStore(ctx, config)
			if err != nil && isStorageTypeError(err) {
				c.logger.Debug(
					"Object Store exists with different storage type, keeping it",
					slog.String("bucket", config.Bucket),
				)
				store, err = c.ExtJS.ObjectStore(ctx, config.Bucket)
			}
			if err != nil {
				return fmt.Errorf(
					"failed to create/update Object Store bucket %s: %w",
					config.Bucket,
					err,
				)
			}
			c.objectStores.Store(config.Bucket, store)

			return nil
		})
}

// objectStore returns the handle of bucket, from the server the first time.
func (c *Client) objectStore(
	ctx context.Context,
	bucket string,
) (jetstream.ObjectStore, error) {
	if store, ok := c.objectStores.Load(bucket); ok {
		return store.(jetstream.ObjectStore), nil
	}
	store, err := c.ExtJS.ObjectStore(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get Object Store bucket %s: %w", bucket, err)
	}
	c.objectStores.Store(bucket, store)

	return store, nil
}

// onObjectStore runs fn on the handle of bucket.
func onObjectStore[T any](
	ctx context.Context,
	c *Client,
	bucket string,
	fn func(jetstream.ObjectStore) (T, error),
) (T, error) {
	store, err := c.objectStore(ctx, bucket)
	if err != nil {
		var zero T
		return zero, err
	}

	return fn(store)
}

// ObjectPut writes the object of meta, read from reader, to bucket.
func (c *Client) ObjectPut(
	ctx context.Context,
	bucket string,
	meta jetstream.ObjectMeta,
	reader io.Reader,
) (*jetstream.ObjectInfo, error) {
	return observe(ctx, c.tel, newOp("objectstore.put", bucket, objectAttr(meta.Name)),
		func(ctx context.Context) (*jetstream.ObjectInfo, error) {
			return onObjectStore(ctx, c, bucket,
				func(store jetstream.ObjectStore) (*jetstream.ObjectInfo, error) {
					return store.Put(ctx, meta, reader)
				})
		})
}

// ObjectPutBytes writes data as the object name to bucket.
func (c *Client) ObjectPutBytes(
	ctx context.Context,
	bucket string,
	name string,
	data []byte,
) (*jetstream.ObjectInfo, error) {
	return observe(ctx, c.tel, newOp("objectstore.put", bucket, objectAttr(name)),
		func(ctx context.Context) (*jetstream.ObjectInfo, error) {
			return onObjectStore(ctx, c, bucket,
				func(store jetstream.ObjectStore) (*jetstream.ObjectInfo, error) {
					return store.PutBytes(ctx, name, data)
				})
		})
}

// ObjectGetBytes returns the content of the object name in bucket. An object
// that is not there returns jetstream.ErrObjectNotFound.
func (c *Client) ObjectGetBytes(
	ctx context.Context,
	bucket string,
	name string,
) ([]byte, error) {
	return observe(ctx, c.tel, newOp("objectstore.get", bucket, objectAttr(name)),
		func(ctx context.Context) ([]byte, error) {
			return onObjectStore(ctx, c, bucket, func(store jetstream.ObjectStore) ([]byte, error) {
				return store.GetBytes(ctx, name)
			})
		})
}

// ObjectInfo returns the information of the object name in bucket, such as
// its size and its digest, without its content.
func (c *Client) ObjectInfo(
	ctx context.Context,
	bucket string,
	name string,
) (*jetstream.ObjectInfo, error) {
	return observe(ctx, c.tel, newOp("objectstore.info", bucket, objectAttr(name)),
		func(ctx context.Context) (*jetstream.ObjectInfo, error) {
			return onObjectStore(ctx, c, bucket,
				func(store jetstream.ObjectStore) (*jetstream.ObjectInfo, error) {
					return store.GetInfo(ctx, name)
				})
		})
}

// ObjectList returns the information of each object in bucket. A bucket with
// no object returns an empty list, not an error.
func (c *Client) ObjectList(
	ctx context.Context,
	bucket string,
) ([]*jetstream.ObjectInfo, error) {
	return observe(ctx, c.tel, newOp("objectstore.list", bucket),
		func(ctx context.Context) ([]*jetstream.ObjectInfo, error) {
			return onObjectStore(ctx, c, bucket,
				func(store jetstream.ObjectStore) ([]*jetstream.ObjectInfo, error) {
					infos, err := store.List(ctx)
					if errors.Is(err, jetstream.ErrNoObjectsFound) {
						return []*jetstream.ObjectInfo{}, nil
					}

					return infos, err
				})
		})
}

// ObjectDelete deletes the object name from bucket.
func (c *Client) ObjectDelete(
	ctx context.Context,
	bucket string,
	name string,
) error {
	return observeErr(ctx, c.tel, newOp("objectstore.delete", bucket, objectAttr(name)),
		func(ctx context.Context) error {
			_, err := onObjectStore(
				ctx,
				c,
				bucket,
				func(store jetstream.ObjectStore) (struct{}, error) {
					return struct{}{}, store.Delete(ctx, name)
				},
			)

			return err
		})
}

// ObjectStoreStatus returns the status of bucket, such as its size.
func (c *Client) ObjectStoreStatus(
	ctx context.Context,
	bucket string,
) (jetstream.ObjectStoreStatus, error) {
	return observe(ctx, c.tel, newOp("objectstore.status", bucket),
		func(ctx context.Context) (jetstream.ObjectStoreStatus, error) {
			return onObjectStore(ctx, c, bucket,
				func(store jetstream.ObjectStore) (jetstream.ObjectStoreStatus, error) {
					return store.Status(ctx)
				})
		})
}

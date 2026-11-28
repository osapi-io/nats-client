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
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
)

// keyAttr is the attribute of the key of a KV call, on its span only.
func keyAttr(
	key string,
) attribute.KeyValue {
	return attribute.String("nats.kv.key", key)
}

// KVCreateOrUpdate creates the KV bucket of config, or updates it to match.
//
// NATS does not allow a change of the storage type of an existing bucket. When
// only that differs, the bucket stays as it is, and the other settings that
// can change are not applied.
func (c *Client) KVCreateOrUpdate(
	ctx context.Context,
	config jetstream.KeyValueConfig,
) error {
	return observeErr(ctx, c.tel, newOp("kv.create_or_update", config.Bucket),
		func(ctx context.Context) error {
			c.logger.Debug("creating/updating KV bucket", slog.String("bucket", config.Bucket))

			js, err := c.JetStream()
			if err != nil {
				return err
			}
			kv, err := js.CreateOrUpdateKeyValue(ctx, config)
			if err != nil && isStorageTypeError(err) {
				c.logger.Debug(
					"KV bucket exists with different storage type, keeping it",
					slog.String("bucket", config.Bucket),
				)
				kv, err = js.KeyValue(ctx, config.Bucket)
			}
			if err != nil {
				return fmt.Errorf("failed to create/update KV bucket %s: %w", config.Bucket, err)
			}
			c.remember(&c.kvs, js, config.Bucket, kv)

			return nil
		})
}

// keyValue returns the handle of bucket, from the server the first time.
func (c *Client) keyValue(
	ctx context.Context,
	bucket string,
) (jetstream.KeyValue, error) {
	if kv, ok := c.kvs.Load(bucket); ok {
		return kv.(jetstream.KeyValue), nil
	}
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}
	kv, err := js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket %s: %w", bucket, err)
	}
	c.remember(&c.kvs, js, bucket, kv)

	return kv, nil
}

// onKV runs fn on the handle of bucket.
func onKV[T any](
	ctx context.Context,
	c *Client,
	bucket string,
	fn func(jetstream.KeyValue) (T, error),
) (T, error) {
	kv, err := c.keyValue(ctx, bucket)
	if err != nil {
		var zero T
		return zero, err
	}
	v, err := fn(kv)

	return v, forget(&c.kvs, bucket, err)
}

// KVGet returns the entry of key in bucket. A key that is not there returns
// jetstream.ErrKeyNotFound.
func (c *Client) KVGet(
	ctx context.Context,
	bucket string,
	key string,
) (jetstream.KeyValueEntry, error) {
	return observe(ctx, c.tel, newOp("kv.get", bucket, keyAttr(key)),
		func(ctx context.Context) (jetstream.KeyValueEntry, error) {
			return onKV(
				ctx,
				c,
				bucket,
				func(kv jetstream.KeyValue) (jetstream.KeyValueEntry, error) {
					return kv.Get(ctx, key)
				},
			)
		})
}

// KVPut writes value to key in bucket, and returns the new revision.
func (c *Client) KVPut(
	ctx context.Context,
	bucket string,
	key string,
	value []byte,
) (uint64, error) {
	return observe(ctx, c.tel, newOp("kv.put", bucket, keyAttr(key)),
		func(ctx context.Context) (uint64, error) {
			return onKV(ctx, c, bucket, func(kv jetstream.KeyValue) (uint64, error) {
				return kv.Put(ctx, key, value)
			})
		})
}

// KVCreate writes value to key in bucket only when the key is not there, and
// returns the revision. A key that is there returns jetstream.ErrKeyExists.
func (c *Client) KVCreate(
	ctx context.Context,
	bucket string,
	key string,
	value []byte,
) (uint64, error) {
	return observe(ctx, c.tel, newOp("kv.create", bucket, keyAttr(key)),
		func(ctx context.Context) (uint64, error) {
			return onKV(ctx, c, bucket, func(kv jetstream.KeyValue) (uint64, error) {
				return kv.Create(ctx, key, value)
			})
		})
}

// KVUpdate writes value to key in bucket only when the key is at revision,
// and returns the new revision.
func (c *Client) KVUpdate(
	ctx context.Context,
	bucket string,
	key string,
	value []byte,
	revision uint64,
) (uint64, error) {
	return observe(ctx, c.tel, newOp("kv.update", bucket, keyAttr(key)),
		func(ctx context.Context) (uint64, error) {
			return onKV(ctx, c, bucket, func(kv jetstream.KeyValue) (uint64, error) {
				return kv.Update(ctx, key, value, revision)
			})
		})
}

// KVDelete deletes key from bucket.
func (c *Client) KVDelete(
	ctx context.Context,
	bucket string,
	key string,
) error {
	return observeErr(ctx, c.tel, newOp("kv.delete", bucket, keyAttr(key)),
		func(ctx context.Context) error {
			_, err := onKV(ctx, c, bucket, func(kv jetstream.KeyValue) (struct{}, error) {
				return struct{}{}, kv.Delete(ctx, key)
			})

			return err
		})
}

// ErrWatchStopped is the error of a KVKeys whose watch stopped before NATS
// sent the end of the keys, such as when the connection dropped. The list is
// not complete, so KVKeys returns no keys with it.
var ErrWatchStopped = errors.New("the watch stopped before the end of the keys")

// KVKeys returns the keys of bucket that match one of filters, such as
// "jobs.>", or each key when there is no filter. A bucket with no key returns
// an empty list, not an error. A list that the end of ctx, or a stop of the
// watch, cut short returns an error, never a short list.
func (c *Client) KVKeys(
	ctx context.Context,
	bucket string,
	filters ...string,
) ([]string, error) {
	return observe(ctx, c.tel, newOp("kv.keys", bucket),
		func(ctx context.Context) ([]string, error) {
			return onKV(ctx, c, bucket, func(kv jetstream.KeyValue) ([]string, error) {
				opts := []jetstream.WatchOpt{jetstream.IgnoreDeletes(), jetstream.MetaOnly()}
				var (
					watcher jetstream.KeyWatcher
					err     error
				)
				if len(filters) == 0 {
					watcher, err = kv.WatchAll(ctx, opts...)
				} else {
					watcher, err = kv.WatchFiltered(ctx, filters, opts...)
				}
				if err != nil {
					return nil, err
				}
				defer func() { _ = watcher.Stop() }()

				return readKeys(ctx, bucket, watcher.Updates())
			})
		})
}

// readKeys reads the keys that updates sends until NATS marks their end
// with a nil entry. A channel that closes before that mark, as it does when
// the connection drops, is ErrWatchStopped, not a short list. The end of
// ctx is ctx's error.
func readKeys(
	ctx context.Context,
	bucket string,
	updates <-chan jetstream.KeyValueEntry,
) ([]string, error) {
	keys := []string{}
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("list the keys of %s: %w", bucket, ctx.Err())
		case entry, ok := <-updates:
			if !ok {
				// nats.go closes the channel when ctx ends too. Then the
				// end of ctx is the reason.
				if err := ctx.Err(); err != nil {
					return nil, fmt.Errorf("list the keys of %s: %w", bucket, err)
				}

				return nil, fmt.Errorf("list the keys of %s: %w", bucket, ErrWatchStopped)
			}
			if entry == nil {
				return keys, nil
			}
			keys = append(keys, entry.Key())
		}
	}
}

// KVWatch sends each change to a key of bucket that matches pattern, such as
// "responses.>" or ">" for each key, on the channel that it returns. The
// current values come first. The channel carries entries only: it does not
// mark the end of the current values, so a caller that needs a full snapshot
// reads the keys with KVKeys and KVGet. The watch stops, and the channel
// closes, when ctx ends. The channel also closes when the watch stops on its
// own, such as when the connection closes. A caller that sees the channel
// close while its ctx is still live knows that the watch ended, and can
// start it again. The span covers the start of the watch.
func (c *Client) KVWatch(
	ctx context.Context,
	bucket string,
	pattern string,
	opts ...jetstream.WatchOpt,
) (<-chan jetstream.KeyValueEntry, error) {
	watcher, err := observe(ctx, c.tel, newOp("kv.watch", bucket, keyAttr(pattern)),
		func(ctx context.Context) (jetstream.KeyWatcher, error) {
			return onKV(ctx, c, bucket, func(kv jetstream.KeyValue) (jetstream.KeyWatcher, error) {
				return kv.Watch(ctx, pattern, opts...)
			})
		})
	if err != nil {
		return nil, err
	}

	out := make(chan jetstream.KeyValueEntry)
	go func() {
		defer close(out)
		defer func() { _ = watcher.Stop() }()

		for {
			select {
			case <-ctx.Done():
				return
			case entry, ok := <-watcher.Updates():
				if !ok {
					return
				}
				// NATS sends nil after the current values. The channel
				// carries entries only.
				if entry == nil {
					continue
				}
				select {
				case out <- entry:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

// KVStatus returns the status of bucket, such as its count of values and its
// size.
func (c *Client) KVStatus(
	ctx context.Context,
	bucket string,
) (jetstream.KeyValueStatus, error) {
	return observe(ctx, c.tel, newOp("kv.status", bucket),
		func(ctx context.Context) (jetstream.KeyValueStatus, error) {
			return onKV(
				ctx,
				c,
				bucket,
				func(kv jetstream.KeyValue) (jetstream.KeyValueStatus, error) {
					return kv.Status(ctx)
				},
			)
		})
}

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
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
)

// jsErrCodeStorageTypeChange is the NATS JetStream error code returned when
// attempting to change the storage type on an existing stream. Storage type
// is immutable once a stream is created.
const jsErrCodeStorageTypeChange = 10052

// readConsumerDeleteWait is the longest that StreamRead waits to delete the
// consumer that it read with.
const readConsumerDeleteWait = 5 * time.Second

// isStorageTypeError returns true if the error is a NATS API error indicating
// that the storage type cannot be changed on an existing stream.
func isStorageTypeError(
	err error,
) bool {
	var apiErr *jetstream.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode == jsErrCodeStorageTypeChange
	}

	return false
}

// StreamCreateOrUpdate creates the stream of config, or updates it to match.
// NATS cannot change the storage type of a stream that exists. Thus, for a
// stream whose storage type differs, it keeps the storage type on the server
// and applies the rest of config.
func (c *Client) StreamCreateOrUpdate(
	ctx context.Context,
	config jetstream.StreamConfig,
) error {
	return observeErr(ctx, c.tel, newOp("stream.create_or_update", config.Name),
		func(ctx context.Context) error {
			c.logger.Debug(
				"creating stream",
				slog.String("name", config.Name),
				slog.String("subjects", strings.Join(config.Subjects, ", ")),
			)

			js, err := c.JetStream()
			if err != nil {
				return err
			}
			stream, err := js.CreateOrUpdateStream(ctx, config)
			if err != nil && isStorageTypeError(err) {
				stream, err = c.updateKeepingStorage(ctx, js, config)
			}
			if err != nil {
				return fmt.Errorf("error creating/updating stream %s: %w", config.Name, err)
			}
			c.remember(&c.streams, js, config.Name, stream)

			return nil
		})
}

// updateKeepingStorage updates the stream of config with the storage type
// that the stream has on the server, which NATS cannot change.
func (c *Client) updateKeepingStorage(
	ctx context.Context,
	js jetstream.JetStream,
	config jetstream.StreamConfig,
) (jetstream.Stream, error) {
	existing, err := js.Stream(ctx, config.Name)
	if err != nil {
		return nil, err
	}

	kept := config
	kept.Storage = existing.CachedInfo().Config.Storage
	c.logger.Warn(
		"stream exists with a different storage type: keeping it, and applying the rest",
		slog.String("name", config.Name),
		slog.String("storage", kept.Storage.String()),
	)

	return js.UpdateStream(ctx, kept)
}

// ConsumerCreateOrUpdate creates the durable consumer of config on stream, or
// updates it to match.
func (c *Client) ConsumerCreateOrUpdate(
	ctx context.Context,
	stream string,
	config jetstream.ConsumerConfig,
) error {
	return observeErr(ctx, c.tel,
		newOp("stream.consumer.create_or_update", stream,
			attribute.String("nats.consumer.name", config.Durable)),
		func(ctx context.Context) error {
			c.logger.Debug(
				"creating consumer",
				slog.String("durable", config.Durable),
				slog.String("stream", stream),
			)

			js, err := c.JetStream()
			if err != nil {
				return err
			}
			if _, err := js.CreateOrUpdateConsumer(ctx, stream, config); err != nil {
				return fmt.Errorf("error creating consumer for stream %s: %w", stream, err)
			}

			return nil
		})
}

// stream returns the handle of the stream name, from the server the first
// time.
func (c *Client) stream(
	ctx context.Context,
	name string,
) (jetstream.Stream, error) {
	if s, ok := c.streams.Load(name); ok {
		return s.(jetstream.Stream), nil
	}
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}
	s, err := js.Stream(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream %s: %w", name, err)
	}
	c.remember(&c.streams, js, name, s)

	return s, nil
}

// onStream runs fn on the handle of the stream name.
func onStream[T any](
	ctx context.Context,
	c *Client,
	name string,
	fn func(jetstream.Stream) (T, error),
) (T, error) {
	s, err := c.stream(ctx, name)
	if err != nil {
		var zero T
		return zero, err
	}
	v, err := fn(s)

	return v, forget(&c.streams, name, err)
}

// StreamInfo returns the configuration and the state of the stream name, such
// as its count of messages and its first sequence. It asks the server each
// time, so the state is current.
func (c *Client) StreamInfo(
	ctx context.Context,
	name string,
) (*jetstream.StreamInfo, error) {
	return observe(ctx, c.tel, newOp("stream.info", name),
		func(ctx context.Context) (*jetstream.StreamInfo, error) {
			return onStream(ctx, c, name, func(s jetstream.Stream) (*jetstream.StreamInfo, error) {
				info, err := s.Info(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to get stream info for %s: %w", name, err)
				}

				return info, nil
			})
		})
}

// StreamLastMsg returns the last message on subject in the stream name.
func (c *Client) StreamLastMsg(
	ctx context.Context,
	name string,
	subject string,
) (*jetstream.RawStreamMsg, error) {
	return observe(ctx, c.tel,
		newOp("stream.last_msg", name, attribute.String("messaging.destination.name", subject)),
		func(ctx context.Context) (*jetstream.RawStreamMsg, error) {
			return onStream(
				ctx,
				c,
				name,
				func(s jetstream.Stream) (*jetstream.RawStreamMsg, error) {
					return s.GetLastMsgForSubject(ctx, subject)
				},
			)
		})
}

// StreamGetMsg returns the message at seq in the stream name, read directly,
// with no consumer. A message that the stream no longer has, such as one
// past its max age, is jetstream.ErrMsgNotFound.
func (c *Client) StreamGetMsg(
	ctx context.Context,
	name string,
	seq uint64,
) (*jetstream.RawStreamMsg, error) {
	return observe(ctx, c.tel,
		newOp("stream.get_msg", name),
		func(ctx context.Context) (*jetstream.RawStreamMsg, error) {
			return onStream(
				ctx,
				c,
				name,
				func(s jetstream.Stream) (*jetstream.RawStreamMsg, error) {
					return s.GetMsg(ctx, seq)
				},
			)
		})
}

// StreamPurgeSubject removes each message of subject from the stream name,
// such as the messages of one key of a KV bucket from its stream KV_<bucket>.
// A purge writes nothing, so it works on a stream that is at its max_bytes,
// where a KV delete, which writes a marker, is refused.
func (c *Client) StreamPurgeSubject(
	ctx context.Context,
	name string,
	subject string,
) error {
	_, err := observe(ctx, c.tel,
		newOp("stream.purge", name, attribute.String("messaging.destination.name", subject)),
		func(ctx context.Context) (struct{}, error) {
			return onStream(ctx, c, name, func(s jetstream.Stream) (struct{}, error) {
				if err := s.Purge(ctx, jetstream.WithPurgeSubject(subject)); err != nil {
					return struct{}{}, fmt.Errorf(
						"failed to purge %s from stream %s: %w",
						subject,
						name,
						err,
					)
				}

				return struct{}{}, nil
			})
		})

	return err
}

// StreamRead reads up to count messages of the stream name in order, from the
// sequence from, or from the first message when from is 0. It returns the
// messages that the stream has now, and does not wait for more. count must be
// 1 or more. When the read stopped with an error, it returns the messages
// that it read with the error. It deletes the consumer that it read with, so
// a read leaves no consumer on the server.
func (c *Client) StreamRead(
	ctx context.Context,
	name string,
	from uint64,
	count int,
) ([]jetstream.Msg, error) {
	return observe(ctx, c.tel,
		newOp("stream.read", name, attribute.Int("nats.message.count", count)),
		func(ctx context.Context) ([]jetstream.Msg, error) {
			if count < 1 {
				return nil, fmt.Errorf("read stream %s: count %d is less than 1", name, count)
			}
			s, err := c.stream(ctx, name)
			if err != nil {
				return nil, err
			}

			cfg := jetstream.OrderedConsumerConfig{DeliverPolicy: jetstream.DeliverAllPolicy}
			if from > 0 {
				cfg = jetstream.OrderedConsumerConfig{
					DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
					OptStartSeq:   from,
				}
			}
			consumer, err := s.OrderedConsumer(ctx, cfg)
			if err != nil {
				return nil, forget(&c.streams, name,
					fmt.Errorf("create ordered consumer on %s: %w", name, err))
			}

			batch, err := consumer.FetchNoWait(count)
			defer c.deleteReadConsumer(ctx, s, consumer)
			if err != nil {
				return nil, fmt.Errorf("read stream %s: %w", name, err)
			}

			msgs := make([]jetstream.Msg, 0, count)
			for msg := range batch.Messages() {
				msgs = append(msgs, msg)
			}
			if err := batch.Error(); err != nil {
				return msgs, fmt.Errorf("read stream %s: %w", name, err)
			}

			return msgs, nil
		})
}

// deleteReadConsumer deletes the ordered consumer that StreamRead read with,
// also when ctx ended, because the server keeps it until its inactive
// threshold. A consumer that has no name yet was never made on the server.
// A failed delete is logged and left to that threshold.
func (c *Client) deleteReadConsumer(
	ctx context.Context,
	s jetstream.Stream,
	consumer jetstream.Consumer,
) {
	info := consumer.CachedInfo()
	if info == nil || info.Name == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), readConsumerDeleteWait)
	defer cancel()
	if err := s.DeleteConsumer(ctx, info.Name); err != nil {
		c.logger.Debug("delete stream read consumer",
			slog.String("stream", info.Stream),
			slog.String("consumer", info.Name),
			slog.String("error", err.Error()),
		)
	}
}

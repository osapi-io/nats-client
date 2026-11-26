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

package client_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type MessagingPublicTestSuite struct {
	suite.Suite

	ctrl   *gomock.Controller
	js     *mocks.MockJetStream
	nc     *mocks.MockNATSConnector
	client *client.Client
	ctx    context.Context
}

func (s *MessagingPublicTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.js = mocks.NewMockJetStream(s.ctrl)
	s.nc = mocks.NewMockNATSConnector(s.ctrl)
	s.client = client.New(slog.Default(), &client.Options{})
	s.client.ExtJS = s.js
	s.client.NC = s.nc
	s.ctx = context.Background()
}

func (s *MessagingPublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *MessagingPublicTestSuite) TestPublish() {
	s.Run("publishes the data to the subject", func() {
		s.js.EXPECT().
			PublishMsg(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, msg *nats.Msg, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
				s.Equal("jobs.x", msg.Subject)
				s.Equal([]byte("data"), msg.Data)

				return &jetstream.PubAck{}, nil
			})

		s.NoError(s.client.Publish(s.ctx, "jobs.x", []byte("data")))
	})

	s.Run("when JetStream refuses returns the error", func() {
		s.js.EXPECT().PublishMsg(gomock.Any(), gomock.Any()).Return(nil, errBoom)

		s.EqualError(
			s.client.Publish(s.ctx, "jobs.x", nil),
			"failed to publish message to jobs.x: boom",
		)
	})

	s.Run("before Connect returns an error", func() {
		s.client.ExtJS = nil

		s.ErrorIs(s.client.Publish(s.ctx, "jobs.x", nil), client.ErrNotConnected)
	})
}

func (s *MessagingPublicTestSuite) TestPublishCore() {
	s.Run("publishes the data to the subject", func() {
		s.nc.EXPECT().PublishMsg(gomock.Any()).DoAndReturn(func(msg *nats.Msg) error {
			s.Equal("enroll.request", msg.Subject)
			s.Equal([]byte("data"), msg.Data)

			return nil
		})

		s.NoError(s.client.PublishCore(s.ctx, "enroll.request", []byte("data")))
	})

	s.Run("when NATS refuses returns the error", func() {
		s.nc.EXPECT().PublishMsg(gomock.Any()).Return(errBoom)

		s.EqualError(
			s.client.PublishCore(s.ctx, "enroll.request", nil),
			"failed to publish to enroll.request: boom",
		)
	})

	s.Run("before Connect returns an error", func() {
		s.client.NC = nil

		s.ErrorContains(
			s.client.PublishCore(s.ctx, "enroll.request", nil),
			"NATS connection not established",
		)
	})
}

func (s *MessagingPublicTestSuite) TestSubscribe() {
	s.Run("calls the handler for each message", func() {
		var deliver nats.MsgHandler
		sub := &nats.Subscription{}
		s.nc.EXPECT().
			Subscribe("enroll.response.x", gomock.Any()).
			DoAndReturn(func(_ string, h nats.MsgHandler) (*nats.Subscription, error) {
				deliver = h
				return sub, nil
			})

		var got *nats.Msg
		gotSub, err := s.client.Subscribe(s.ctx, "enroll.response.x",
			func(ctx context.Context, msg *nats.Msg) {
				s.NotNil(ctx)
				got = msg
			})
		s.Require().NoError(err)
		s.Same(sub, gotSub)

		msg := &nats.Msg{Subject: "enroll.response.x", Data: []byte("ok")}
		deliver(msg)
		s.Same(msg, got)
	})

	s.Run("a handler that panics does not stop the process", func() {
		var deliver nats.MsgHandler
		s.nc.EXPECT().
			Subscribe("x", gomock.Any()).
			DoAndReturn(func(_ string, h nats.MsgHandler) (*nats.Subscription, error) {
				deliver = h
				return &nats.Subscription{}, nil
			})

		_, err := s.client.Subscribe(s.ctx, "x", func(context.Context, *nats.Msg) {
			panic("handler bug")
		})
		s.Require().NoError(err)
		s.NotPanics(func() { deliver(&nats.Msg{Subject: "x"}) })
	})

	s.Run("when NATS refuses returns the error", func() {
		s.nc.EXPECT().Subscribe("x", gomock.Any()).Return(nil, errBoom)

		_, err := s.client.Subscribe(s.ctx, "x", func(context.Context, *nats.Msg) {})
		s.EqualError(err, "failed to subscribe to x: boom")
	})

	s.Run("before Connect returns an error", func() {
		s.client.NC = nil

		_, err := s.client.Subscribe(s.ctx, "x", func(context.Context, *nats.Msg) {})
		s.ErrorContains(err, "NATS connection not established")
	})
}

// msg returns a message on subject that expects one reply: an Ack, a Nak or
// a Term, as reply names, which returns replyErr.
func (s *MessagingPublicTestSuite) msg(
	subject string,
	reply string,
	replyErr error,
) *mocks.MockMsg {
	m := mocks.NewMockMsg(s.ctrl)
	m.EXPECT().Subject().Return(subject).AnyTimes()
	m.EXPECT().Headers().Return(nats.Header{}).AnyTimes()
	switch reply {
	case "ack":
		m.EXPECT().Ack().Return(replyErr)
	case "nak":
		m.EXPECT().Nak().Return(replyErr)
	case "term":
		m.EXPECT().Term().Return(replyErr)
	}

	return m
}

// batchOf returns a batch of msgs, whose Error is err.
func (s *MessagingPublicTestSuite) batchOf(
	err error,
	msgs ...jetstream.Msg,
) *mocks.MockMessageBatch {
	ch := make(chan jetstream.Msg, len(msgs))
	for _, m := range msgs {
		ch <- m
	}
	close(ch)
	batch := mocks.NewMockMessageBatch(s.ctrl)
	batch.EXPECT().Messages().Return(ch)
	batch.EXPECT().Error().Return(err)

	return batch
}

func (s *MessagingPublicTestSuite) TestConsume() {
	s.Run("acknowledges, negatively acknowledges or ends each message by the handler", func() {
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()

		batch := s.batchOf(nil,
			s.msg("ok", "ack", nil),
			s.msg("ack-fails", "ack", errBoom),
			s.msg("fails", "nak", nil),
			s.msg("nak-fails", "nak", errBoom),
			s.msg("terminate", "term", nil),
			s.msg("panics", "nak", nil),
		)
		consumer := mocks.NewMockConsumer(s.ctrl)
		gomock.InOrder(
			consumer.EXPECT().Fetch(6, gomock.Any()).Return(nil, nats.ErrTimeout),
			consumer.EXPECT().Fetch(6, gomock.Any()).Return(nil, errBoom),
			consumer.EXPECT().
				Fetch(6, gomock.Any()).
				DoAndReturn(func(int, ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
					// The loop stops after this batch.
					cancel()
					return batch, nil
				}),
		)
		s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(consumer, nil)

		var mu sync.Mutex
		var handled []string
		err := s.client.Consume(ctx, "S", "c", func(_ context.Context, m jetstream.Msg) error {
			mu.Lock()
			handled = append(handled, m.Subject())
			mu.Unlock()
			switch m.Subject() {
			case "fails", "nak-fails":
				return errBoom
			case "terminate":
				return fmt.Errorf("bad job: %w", client.ErrTerminate)
			case "panics":
				panic("handler bug")
			}

			return nil
		}, &client.ConsumeOptions{MaxInFlight: 6})

		s.ErrorIs(err, context.Canceled)
		// Consume returns only after each handler returned.
		s.ElementsMatch(
			[]string{"ok", "ack-fails", "fails", "nak-fails", "terminate", "panics"},
			handled,
		)
	})

	s.Run("waits longer after each failed fetch", func() {
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()

		consumer := mocks.NewMockConsumer(s.ctrl)
		var times []time.Time
		consumer.EXPECT().
			Fetch(gomock.Any(), gomock.Any()).
			DoAndReturn(func(int, ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
				times = append(times, time.Now())
				if len(times) == 3 {
					cancel()
				}
				return nil, errBoom
			}).
			Times(3)
		s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(consumer, nil)

		err := s.client.Consume(ctx, "S", "c", func(context.Context, jetstream.Msg) error {
			return nil
		}, nil)

		s.ErrorIs(err, context.Canceled)
		s.GreaterOrEqual(times[1].Sub(times[0]), 100*time.Millisecond)
		s.GreaterOrEqual(times[2].Sub(times[1]), 200*time.Millisecond)
	})

	s.Run("a failed batch is a failed fetch, and a batch that times out is not", func() {
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()

		consumer := mocks.NewMockConsumer(s.ctrl)
		gomock.InOrder(
			consumer.EXPECT().Fetch(1, gomock.Any()).Return(s.batchOf(nats.ErrTimeout), nil),
			consumer.EXPECT().Fetch(1, gomock.Any()).Return(s.batchOf(errBoom), nil),
			consumer.EXPECT().
				Fetch(1, gomock.Any()).
				DoAndReturn(func(int, ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
					cancel()
					return nil, context.Canceled
				}),
		)
		s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(consumer, nil)

		err := s.client.Consume(ctx, "S", "c", func(context.Context, jetstream.Msg) error {
			return nil
		}, &client.ConsumeOptions{MaxInFlight: 1})
		s.ErrorIs(err, context.Canceled)
	})

	s.Run("when ctx ends in the wait after a failed fetch returns at once", func() {
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()

		consumer := mocks.NewMockConsumer(s.ctrl)
		consumer.EXPECT().
			Fetch(gomock.Any(), gomock.Any()).
			DoAndReturn(func(int, ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
				time.AfterFunc(20*time.Millisecond, cancel)
				return nil, errBoom
			})
		s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(consumer, nil)

		err := s.client.Consume(ctx, "S", "c", func(context.Context, jetstream.Msg) error {
			return nil
		}, nil)
		s.ErrorIs(err, context.Canceled)
	})

	s.Run("a failure that a retry cannot fix ends Consume", func() {
		for _, fetchErr := range []error{
			nats.ErrConnectionClosed,
			jetstream.ErrConsumerNotFound,
			jetstream.ErrConsumerDeleted,
			jetstream.ErrStreamNotFound,
			jetstream.ErrNoStreamResponse,
		} {
			consumer := mocks.NewMockConsumer(s.ctrl)
			consumer.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(nil, fetchErr)
			s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(consumer, nil)

			err := s.client.Consume(s.ctx, "S", "c", func(context.Context, jetstream.Msg) error {
				return nil
			}, nil)
			s.ErrorIs(err, fetchErr)
			s.ErrorContains(err, "failed to fetch from consumer c")
		}
	})

	s.Run("when ctx ends while each handler runs returns after they return", func() {
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()

		consumer := mocks.NewMockConsumer(s.ctrl)
		consumer.EXPECT().
			Fetch(1, gomock.Any()).
			Return(s.batchOf(nil, s.msg("slow", "ack", nil)), nil)
		s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(consumer, nil)

		started := make(chan struct{})
		release := make(chan struct{})
		go func() {
			<-started
			// Consume now waits for the slot of the handler.
			time.Sleep(20 * time.Millisecond)
			cancel()
			time.Sleep(20 * time.Millisecond)
			close(release)
		}()

		err := s.client.Consume(ctx, "S", "c", func(context.Context, jetstream.Msg) error {
			close(started)
			<-release
			return nil
		}, &client.ConsumeOptions{MaxInFlight: 1})
		s.ErrorIs(err, context.Canceled)
	})

	s.Run("with no options uses the defaults", func() {
		ctx, cancel := context.WithCancel(s.ctx)
		cancel()
		s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(mocks.NewMockConsumer(s.ctrl), nil)

		err := s.client.Consume(ctx, "S", "c", func(context.Context, jetstream.Msg) error {
			return nil
		}, nil)
		s.ErrorIs(err, context.Canceled)
	})

	s.Run("when the consumer is not there returns the error", func() {
		s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(nil, errBoom)

		err := s.client.Consume(s.ctx, "S", "c", func(context.Context, jetstream.Msg) error {
			return nil
		}, nil)
		s.EqualError(err, "failed to get consumer c from stream S: boom")
	})
}

func TestMessagingPublicTestSuite(t *testing.T) {
	suite.Run(t, new(MessagingPublicTestSuite))
}

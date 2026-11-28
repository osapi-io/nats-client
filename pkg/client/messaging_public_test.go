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

package client_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
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
	// started is signalled when a mock call begins, for a row that acts
	// while the call waits.
	started chan struct{}
}

func (s *MessagingPublicTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.js = mocks.NewMockJetStream(s.ctrl)
	s.nc = mocks.NewMockNATSConnector(s.ctrl)
	s.client = client.New(slog.Default(), &client.Options{})
	client.SetJetStream(s.client, s.js)
	s.client.NC = s.nc
	s.ctx = context.Background()
	s.started = make(chan struct{}, 1)
}

func (s *MessagingPublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *MessagingPublicTestSuite) TestPublish() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "publishes the data to the subject",
			validateFunc: func() {
				s.js.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, msg *nats.Msg, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
						s.Equal("jobs.x", msg.Subject)
						s.Equal([]byte("data"), msg.Data)

						return &jetstream.PubAck{}, nil
					})

				s.NoError(s.client.Publish(s.ctx, "jobs.x", []byte("data")))
			},
		},
		{
			name: "when JetStream refuses returns the error",
			validateFunc: func() {
				s.js.EXPECT().PublishMsg(gomock.Any(), gomock.Any()).Return(nil, errBoom)

				s.EqualError(
					s.client.Publish(s.ctx, "jobs.x", nil),
					"failed to publish message to jobs.x: boom",
				)
			},
		},
		{
			name: "before Connect returns an error",
			validateFunc: func() {
				client.SetJetStream(s.client, nil)

				s.ErrorIs(s.client.Publish(s.ctx, "jobs.x", nil), client.ErrNotConnected)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *MessagingPublicTestSuite) TestPublishCore() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "publishes the data to the subject",
			validateFunc: func() {
				s.nc.EXPECT().PublishMsg(gomock.Any()).DoAndReturn(func(msg *nats.Msg) error {
					s.Equal("enroll.request", msg.Subject)
					s.Equal([]byte("data"), msg.Data)

					return nil
				})

				s.NoError(s.client.PublishCore(s.ctx, "enroll.request", []byte("data")))
			},
		},
		{
			name: "when NATS refuses returns the error",
			validateFunc: func() {
				s.nc.EXPECT().PublishMsg(gomock.Any()).Return(errBoom)

				s.EqualError(
					s.client.PublishCore(s.ctx, "enroll.request", nil),
					"failed to publish to enroll.request: boom",
				)
			},
		},
		{
			name: "before Connect returns ErrNotConnected",
			validateFunc: func() {
				fresh := client.New(slog.New(slog.DiscardHandler), &client.Options{})

				s.ErrorIs(fresh.PublishCore(s.ctx, "enroll.request", nil), client.ErrNotConnected)
			},
		},
		{
			name: "a ctx that ended publishes nothing",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(s.ctx)
				cancel()

				s.ErrorIs(s.client.PublishCore(ctx, "enroll.request", nil), context.Canceled)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *MessagingPublicTestSuite) TestRequest() {
	tests := []struct {
		name         string
		fresh        bool
		closeDuring  bool
		ctx          func() (context.Context, context.CancelFunc)
		setup        func()
		validateFunc func(*nats.Msg, error)
	}{
		{
			name: "sends the data and returns the reply, within the default timeout",
			ctx:  func() (context.Context, context.CancelFunc) { return s.ctx, func() {} },
			setup: func() {
				s.nc.EXPECT().RequestMsgWithContext(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, msg *nats.Msg) (*nats.Msg, error) {
						s.Equal("join.request", msg.Subject)
						s.Equal([]byte("data"), msg.Data)
						_, ok := ctx.Deadline()
						s.True(ok)

						return &nats.Msg{Data: []byte("reply")}, nil
					},
				)
			},
			validateFunc: func(m *nats.Msg, err error) {
				s.Require().NoError(err)
				s.Equal([]byte("reply"), m.Data)
			},
		},
		{
			name: "keeps the deadline of ctx",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(s.ctx, time.Minute)
			},
			setup: func() {
				s.nc.EXPECT().RequestMsgWithContext(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, _ *nats.Msg) (*nats.Msg, error) {
						d, _ := ctx.Deadline()
						s.Greater(time.Until(d), client.DefaultRequestTimeout)

						return &nats.Msg{}, nil
					},
				)
			},
			validateFunc: func(_ *nats.Msg, err error) { s.NoError(err) },
		},
		{
			name: "when NATS gives no reply returns the error",
			ctx:  func() (context.Context, context.CancelFunc) { return s.ctx, func() {} },
			setup: func() {
				s.nc.EXPECT().RequestMsgWithContext(gomock.Any(), gomock.Any()).Return(nil, errBoom)
			},
			validateFunc: func(_ *nats.Msg, err error) {
				s.EqualError(err, "failed to request join.request: boom")
			},
		},
		{
			name: "a ctx that ended sends nothing",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(s.ctx)
				cancel()

				return ctx, cancel
			},
			setup:        func() {},
			validateFunc: func(_ *nats.Msg, err error) { s.ErrorIs(err, context.Canceled) },
		},
		{
			name:        "a Close during the wait returns at once and ends the request",
			closeDuring: true,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(s.ctx, 2*time.Second)
			},
			setup: func() {
				closed := make(chan struct{})
				s.nc.EXPECT().RequestMsgWithContext(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, _ *nats.Msg) (*nats.Msg, error) {
						s.started <- struct{}{}
						select {
						case <-closed:
							return nil, nats.ErrConnectionClosed
						case <-ctx.Done():
							return nil, ctx.Err()
						}
					},
				)
				s.nc.EXPECT().Close().Do(func() { close(closed) })
			},
			validateFunc: func(_ *nats.Msg, err error) {
				s.ErrorIs(err, nats.ErrConnectionClosed)
			},
		},
		{
			name:         "before Connect returns ErrNotConnected",
			fresh:        true,
			ctx:          func() (context.Context, context.CancelFunc) { return s.ctx, func() {} },
			setup:        func() {},
			validateFunc: func(_ *nats.Msg, err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			ctx, cancel := tt.ctx()
			defer cancel()
			tt.setup()
			c := s.client
			if tt.fresh {
				c = client.New(slog.New(slog.DiscardHandler), &client.Options{})
			}
			if !tt.closeDuring {
				tt.validateFunc(c.Request(ctx, "join.request", []byte("data")))

				return
			}
			done := make(chan error, 1)
			go func() {
				_, err := c.Request(ctx, "join.request", []byte("data"))
				done <- err
			}()
			<-s.started
			began := time.Now()
			c.Close()
			s.Less(time.Since(began), time.Second)
			tt.validateFunc(nil, <-done)
		})
	}
}

func (s *MessagingPublicTestSuite) TestSubscribe() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "calls the handler for each message",
			validateFunc: func() {
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
			},
		},
		{
			name: "a handler that panics does not stop the process",
			validateFunc: func() {
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
			},
		},
		{
			name: "when NATS refuses returns the error",
			validateFunc: func() {
				s.nc.EXPECT().Subscribe("x", gomock.Any()).Return(nil, errBoom)

				_, err := s.client.Subscribe(s.ctx, "x", func(context.Context, *nats.Msg) {})
				s.EqualError(err, "failed to subscribe to x: boom")
			},
		},
		{
			name: "before Connect returns ErrNotConnected",
			validateFunc: func() {
				fresh := client.New(slog.New(slog.DiscardHandler), &client.Options{})

				_, err := fresh.Subscribe(s.ctx, "x", func(context.Context, *nats.Msg) {})
				s.ErrorIs(err, client.ErrNotConnected)
			},
		},
		{
			name: "with a ctx that ended subscribes nothing",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(s.ctx)
				cancel()

				_, err := s.client.Subscribe(ctx, "x", func(context.Context, *nats.Msg) {})
				s.ErrorIs(err, context.Canceled)
				s.EqualError(err, "failed to subscribe to x: context canceled")
			},
		},
		{
			name: "the end of ctx unsubscribes",
			validateFunc: func() {
				logs := &syncBuffer{}
				c := s.realClient(logs)
				ctx, cancel := context.WithCancel(s.ctx)

				sub, err := c.Subscribe(ctx, "x", func(context.Context, *nats.Msg) {})
				s.Require().NoError(err)
				s.True(sub.IsValid())
				cancel()

				s.Eventually(func() bool { return !sub.IsValid() }, time.Second, time.Millisecond)
				s.Contains(logs.String(), "unsubscribing at the end of ctx")
			},
		},
		{
			name: "an unsubscribe by hand stops the unsubscribe at the end of ctx",
			validateFunc: func() {
				logs := &syncBuffer{}
				c := s.realClient(logs)
				ctx, cancel := context.WithCancel(s.ctx)

				sub, err := c.Subscribe(ctx, "x", func(context.Context, *nats.Msg) {})
				s.Require().NoError(err)
				s.Require().NoError(sub.Unsubscribe())
				// NATS runs the closed handler in the goroutine of the
				// subscription, after the unsubscribe returns.
				s.Eventually(func() bool {
					return strings.Contains(
						logs.String(),
						"subscription closed before the end of ctx",
					)
				}, time.Second, time.Millisecond)
				cancel()

				// The watch of ctx stopped with the subscription, so the end of
				// ctx runs nothing.
				s.Never(func() bool {
					return strings.Contains(logs.String(), "unsubscribing at the end of ctx")
				}, 200*time.Millisecond, 5*time.Millisecond)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

// realClient is a client that is connected to a NATS server that runs for
// the test.
func (s *MessagingPublicTestSuite) realClient(
	logs io.Writer,
) *client.Client {
	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1})
	s.Require().NoError(err)
	srv.Start()
	s.Require().True(srv.ReadyForConnections(5 * time.Second))
	s.T().Cleanup(srv.Shutdown)

	c := client.New(
		slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		&client.Options{
			Host: "127.0.0.1",
			Port: srv.Addr().(*net.TCPAddr).Port,
		},
	)
	s.Require().NoError(c.Connect(context.Background()))
	s.T().Cleanup(c.Close)

	return c
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
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "acknowledges, negatively acknowledges or ends each message by the handler",
			validateFunc: func() {
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
				err := s.client.Consume(
					ctx,
					"S",
					"c",
					func(_ context.Context, m jetstream.Msg) error {
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
					},
					&client.ConsumeOptions{MaxInFlight: 6},
				)

				s.ErrorIs(err, context.Canceled)
				// Consume returns only after each handler returned.
				s.ElementsMatch(
					[]string{"ok", "ack-fails", "fails", "nak-fails", "terminate", "panics"},
					handled,
				)
			},
		},
		{
			name: "when ctx ends while each slot is in use returns at once",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(s.ctx)
				defer cancel()

				consumer := mocks.NewMockConsumer(s.ctrl)
				consumer.EXPECT().
					Fetch(1, gomock.Any()).
					Return(s.batchOf(nil, s.msg("jobs.a", "ack", nil)), nil).
					Times(1)
				s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(consumer, nil)

				started := make(chan struct{})
				done := make(chan error, 1)
				go func() {
					// The handler holds the one slot until ctx ends, so Consume
					// waits in the select for a free slot or the end of ctx, and
					// the end of ctx comes first.
					done <- s.client.Consume(ctx, "S", "c", func(ctx context.Context, _ jetstream.Msg) error {
						close(started)
						<-ctx.Done()

						return nil
					}, &client.ConsumeOptions{MaxInFlight: 1})
				}()

				<-started
				cancel()

				s.ErrorIs(<-done, context.Canceled)
			},
		},
		{
			name: "waits longer after each failed fetch",
			validateFunc: func() {
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
			},
		},
		{
			name: "a failed batch is a failed fetch, and a batch that times out is not",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(s.ctx)
				defer cancel()

				consumer := mocks.NewMockConsumer(s.ctrl)
				gomock.InOrder(
					consumer.EXPECT().
						Fetch(1, gomock.Any()).
						Return(s.batchOf(nats.ErrTimeout), nil),
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
			},
		},
		{
			name: "when ctx ends in the wait after a failed fetch returns at once",
			validateFunc: func() {
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
			},
		},
		{
			name: "a failure that a retry cannot fix ends Consume",
			validateFunc: func() {
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

					err := s.client.Consume(
						s.ctx,
						"S",
						"c",
						func(context.Context, jetstream.Msg) error {
							return nil
						},
						nil,
					)
					s.ErrorIs(err, fetchErr)
					s.ErrorContains(err, "failed to fetch from consumer c")
				}
			},
		},
		{
			name: "when ctx ends while each handler runs returns after they return",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(s.ctx)
				defer cancel()

				consumer := mocks.NewMockConsumer(s.ctrl)
				consumer.EXPECT().
					Fetch(1, gomock.Any()).
					Return(s.batchOf(nil, s.msg("slow", "ack", nil)), nil)
				s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(consumer, nil)

				started := make(chan struct{})
				go func() {
					<-started
					cancel()
				}()

				// The handler runs until ctx ends. Consume returns only after
				// it returns.
				returned := false
				err := s.client.Consume(
					ctx,
					"S",
					"c",
					func(ctx context.Context, _ jetstream.Msg) error {
						close(started)
						<-ctx.Done()
						returned = true

						return nil
					},
					&client.ConsumeOptions{MaxInFlight: 1},
				)
				s.ErrorIs(err, context.Canceled)
				s.True(returned)
			},
		},
		{
			name: "with no options uses the defaults",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(s.ctx)
				cancel()
				s.js.EXPECT().
					Consumer(gomock.Any(), "S", "c").
					Return(mocks.NewMockConsumer(s.ctrl), nil)

				err := s.client.Consume(ctx, "S", "c", func(context.Context, jetstream.Msg) error {
					return nil
				}, nil)
				s.ErrorIs(err, context.Canceled)
			},
		},
		{
			name: "when the consumer is not there returns the error",
			validateFunc: func() {
				s.js.EXPECT().Consumer(gomock.Any(), "S", "c").Return(nil, errBoom)

				err := s.client.Consume(
					s.ctx,
					"S",
					"c",
					func(context.Context, jetstream.Msg) error {
						return nil
					},
					nil,
				)
				s.EqualError(err, "failed to get consumer c from stream S: boom")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func TestMessagingPublicTestSuite(t *testing.T) {
	suite.Run(t, new(MessagingPublicTestSuite))
}

// ConsumeServerPublicTestSuite runs Consume against a real NATS server,
// because only a real pull request shows how long a wait for a message takes.
type ConsumeServerPublicTestSuite struct {
	suite.Suite

	server *server.Server
	client *client.Client
}

func (s *ConsumeServerPublicTestSuite) SetupTest() {
	srv, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  s.T().TempDir(),
	})
	s.Require().NoError(err)
	srv.Start()
	s.Require().True(srv.ReadyForConnections(5 * time.Second))
	s.server = srv

	port := srv.Addr().(*net.TCPAddr).Port
	s.client = client.New(slog.Default(), &client.Options{Host: "127.0.0.1", Port: port})
	s.Require().NoError(s.client.Connect(context.Background()))

	ctx := context.Background()
	s.Require().NoError(s.client.StreamCreateOrUpdate(ctx, jetstream.StreamConfig{
		Name:     "JOBS",
		Subjects: []string{"jobs.>"},
		Storage:  jetstream.MemoryStorage,
	}))
	s.Require().NoError(s.client.ConsumerCreateOrUpdate(ctx, "JOBS", jetstream.ConsumerConfig{
		Durable:   "agent",
		AckPolicy: jetstream.AckExplicitPolicy,
	}))
}

func (s *ConsumeServerPublicTestSuite) TearDownTest() {
	s.client.Close()
	s.server.Shutdown()
}

func (s *ConsumeServerPublicTestSuite) TestConsume() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "gives a published message to the handler",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				got := make(chan string, 1)
				done := make(chan error, 1)
				go func() {
					done <- s.client.Consume(ctx, "JOBS", "agent",
						func(_ context.Context, msg jetstream.Msg) error {
							got <- string(msg.Data())
							return nil
						}, nil)
				}()

				s.Require().
					NoError(s.client.Publish(context.Background(), "jobs.x", []byte("job-1")))
				select {
				case data := <-got:
					s.Equal("job-1", data)
				case <-time.After(5 * time.Second):
					s.Fail("the handler did not get the message")
				}
				cancel()
				s.ErrorIs(<-done, context.Canceled)
			},
		},
		{
			name: "handles up to MaxInFlight messages at the same time",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				var mu sync.Mutex
				running, most, handled := 0, 0, 0
				release := make(chan struct{})
				done := make(chan error, 1)
				go func() {
					done <- s.client.Consume(ctx, "JOBS", "agent",
						func(context.Context, jetstream.Msg) error {
							mu.Lock()
							running++
							most = max(most, running)
							mu.Unlock()
							<-release
							mu.Lock()
							running--
							handled++
							mu.Unlock()
							return nil
						}, &client.ConsumeOptions{MaxInFlight: 2})
				}()

				for i := range 5 {
					s.Require().NoError(
						s.client.Publish(context.Background(), "jobs.x", fmt.Appendf(nil, "job-%d", i)),
					)
				}
				s.Eventually(func() bool {
					mu.Lock()
					defer mu.Unlock()
					return running == 2
				}, 5*time.Second, 10*time.Millisecond)
				close(release)
				s.Eventually(func() bool {
					mu.Lock()
					defer mu.Unlock()
					return handled == 5
				}, 5*time.Second, 10*time.Millisecond)
				cancel()
				s.ErrorIs(<-done, context.Canceled)
				s.Equal(2, most)
			},
		},
		{
			name: "when ctx ends while it waits for a message returns at once",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() {
					done <- s.client.Consume(ctx, "JOBS", "agent",
						func(context.Context, jetstream.Msg) error { return nil }, nil)
				}()

				// Wait until the server has the pull request of Consume. It
				// expires after 30 seconds, so a return within 2 seconds shows
				// that ctx ended it.
				js, err := s.client.JetStream()
				s.Require().NoError(err)
				s.Eventually(func() bool {
					cons, err := js.Consumer(context.Background(), "JOBS", "agent")
					if err != nil {
						return false
					}
					info, err := cons.Info(context.Background())

					return err == nil && info.NumWaiting > 0
				}, 5*time.Second, 10*time.Millisecond)
				began := time.Now()
				cancel()

				select {
				case err := <-done:
					s.ErrorIs(err, context.Canceled)
					s.Less(time.Since(began), 2*time.Second)
				case <-time.After(5 * time.Second):
					s.Fail("Consume did not return when ctx ended")
				}
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func TestConsumeServerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConsumeServerPublicTestSuite))
}

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
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
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

type ConnectionPublicTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockNATS *mocks.MockNATSConnector
	mockJS   *mocks.MockJetStream
	client   *client.Client
}

func (s *ConnectionPublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockNATS = mocks.NewMockNATSConnector(s.mockCtrl)
	s.mockJS = mocks.NewMockJetStream(s.mockCtrl)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	s.client = client.New(logger, &client.Options{
		Host: "localhost",
		Port: 4222,
	})
	s.client.NC = s.mockNATS
	client.SetJetStream(s.client, s.mockJS)
}

func (s *ConnectionPublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *ConnectionPublicTestSuite) TestClose() {
	// kv is the bucket handle that the first row reads before and after Close.
	var kv *mocks.MockKeyValue

	tests := []struct {
		name         string
		setupFunc    func()
		validateFunc func()
	}{
		{
			name: "a call after Close is ErrNotConnected, and a new connection asks for each handle again",
			setupFunc: func() {
				ctx := context.Background()
				kv = mocks.NewMockKeyValue(s.mockCtrl)
				kv.EXPECT().Get(gomock.Any(), "k").Return(nil, jetstream.ErrKeyNotFound).Times(2)
				s.mockJS.EXPECT().KeyValue(gomock.Any(), "b").Return(kv, nil)
				_, err := s.client.KVGet(ctx, "b", "k")
				s.ErrorIs(err, jetstream.ErrKeyNotFound)

				s.mockNATS.EXPECT().Close()
			},
			validateFunc: func() {
				ctx := context.Background()
				s.Nil(client.JetStreamOf(s.client))
				_, err := s.client.KVGet(ctx, "b", "k")
				s.ErrorIs(err, client.ErrNotConnected)

				js := mocks.NewMockJetStream(s.mockCtrl)
				js.EXPECT().KeyValue(gomock.Any(), "b").Return(kv, nil)
				client.SetJetStream(s.client, js)
				_, err = s.client.KVGet(ctx, "b", "k")
				s.ErrorIs(err, jetstream.ErrKeyNotFound)
			},
		},
		{
			name: "closes the underlying connection",
			setupFunc: func() {
				s.mockNATS.EXPECT().Close()
			},
		},
		{
			name: "handles nil NC gracefully",
			setupFunc: func() {
				s.client.NC = nil
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.setupFunc()
			s.client.Close()
			if tc.validateFunc != nil {
				tc.validateFunc()
			}
		})
	}
}

func (s *ConnectionPublicTestSuite) TestCloseDuringCalls() {
	tests := []struct {
		name    string
		readers int
		calls   int
	}{
		{
			name:    "a call that runs while the client closes returns its result or ErrNotConnected",
			readers: 8,
			calls:   200,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			ctx := context.Background()
			s.mockJS.EXPECT().
				KeyValue(gomock.Any(), "b").
				Return(nil, jetstream.ErrBucketNotFound).
				AnyTimes()
			s.mockNATS.EXPECT().Close().AnyTimes()

			var wg, started sync.WaitGroup
			errs := make(chan error, tc.readers*tc.calls)
			started.Add(tc.readers)
			for range tc.readers {
				wg.Go(func() {
					for i := range tc.calls {
						_, err := s.client.KVStatus(ctx, "b")
						errs <- err
						if i == 0 {
							started.Done()
						}
					}
				})
			}
			started.Wait()
			s.client.Close()
			wg.Wait()
			close(errs)

			for err := range errs {
				if !errors.Is(err, client.ErrNotConnected) {
					s.ErrorIs(err, jetstream.ErrBucketNotFound)
				}
			}
			_, err := s.client.KVStatus(ctx, "b")
			s.ErrorIs(err, client.ErrNotConnected)
		})
	}
}

func (s *ConnectionPublicTestSuite) TestNotConnected() {
	ctx := context.Background()
	tests := []struct {
		name         string
		call         func() error
		validateFunc func(error)
	}{
		{
			name:         "Publish",
			call:         func() error { return s.client.Publish(ctx, "x", nil) },
			validateFunc: func(err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
		{
			name: "Consume",
			call: func() error {
				return s.client.Consume(ctx, "S", "c",
					func(context.Context, jetstream.Msg) error { return nil }, nil)
			},
			validateFunc: func(err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
		{
			name: "KVCreateOrUpdate",
			call: func() error {
				return s.client.KVCreateOrUpdate(ctx, jetstream.KeyValueConfig{Bucket: "b"})
			},
			validateFunc: func(err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
		{
			name:         "KVGet",
			call:         func() error { _, err := s.client.KVGet(ctx, "b", "k"); return err },
			validateFunc: func(err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
		{
			name: "ObjectStoreCreateOrUpdate",
			call: func() error {
				return s.client.ObjectStoreCreateOrUpdate(
					ctx, jetstream.ObjectStoreConfig{Bucket: "b"})
			},
			validateFunc: func(err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
		{
			name:         "ObjectGetBytes",
			call:         func() error { _, err := s.client.ObjectGetBytes(ctx, "b", "o"); return err },
			validateFunc: func(err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
		{
			name: "StreamCreateOrUpdate",
			call: func() error {
				return s.client.StreamCreateOrUpdate(ctx, jetstream.StreamConfig{Name: "S"})
			},
			validateFunc: func(err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
		{
			name: "ConsumerCreateOrUpdate",
			call: func() error {
				return s.client.ConsumerCreateOrUpdate(ctx, "S", jetstream.ConsumerConfig{})
			},
			validateFunc: func(err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
		{
			name:         "StreamInfo",
			call:         func() error { _, err := s.client.StreamInfo(ctx, "S"); return err },
			validateFunc: func(err error) { s.ErrorIs(err, client.ErrNotConnected) },
		},
	}

	for _, tc := range tests {
		s.Run(tc.name+" before Connect returns ErrNotConnected", func() {
			client.SetJetStream(s.client, nil)
			tc.validateFunc(tc.call())
		})
	}
}

func (s *ConnectionPublicTestSuite) TestGoneHandle() {
	ctx := context.Background()

	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "a bucket that is gone is asked for again at the next call",
			validateFunc: func() {
				kv := mocks.NewMockKeyValue(s.mockCtrl)
				gomock.InOrder(
					kv.EXPECT().Get(gomock.Any(), "k").Return(nil, jetstream.ErrNoStreamResponse),
					kv.EXPECT().Get(gomock.Any(), "k").Return(nil, jetstream.ErrKeyNotFound),
				)
				s.mockJS.EXPECT().KeyValue(gomock.Any(), "b").Return(kv, nil).Times(2)

				_, err := s.client.KVGet(ctx, "b", "k")
				s.ErrorIs(err, jetstream.ErrNoStreamResponse)
				_, err = s.client.KVGet(ctx, "b", "k")
				s.ErrorIs(err, jetstream.ErrKeyNotFound)
			},
		},
		{
			name: "an object store that is gone is asked for again at the next call",
			validateFunc: func() {
				store := mocks.NewMockObjectStore(s.mockCtrl)
				gomock.InOrder(
					store.EXPECT().
						GetBytes(gomock.Any(), "o").
						Return(nil, jetstream.ErrBucketNotFound),
					store.EXPECT().GetBytes(gomock.Any(), "o").Return([]byte("x"), nil),
				)
				s.mockJS.EXPECT().ObjectStore(gomock.Any(), "b").Return(store, nil).Times(2)

				_, err := s.client.ObjectGetBytes(ctx, "b", "o")
				s.ErrorIs(err, jetstream.ErrBucketNotFound)
				got, err := s.client.ObjectGetBytes(ctx, "b", "o")
				s.Require().NoError(err)
				s.Equal([]byte("x"), got)
			},
		},
		{
			name: "a stream that is gone is asked for again at the next call",
			validateFunc: func() {
				stream := mocks.NewMockStream(s.mockCtrl)
				gomock.InOrder(
					stream.EXPECT().Info(gomock.Any()).Return(nil, jetstream.ErrStreamNotFound),
					stream.EXPECT().
						OrderedConsumer(gomock.Any(), gomock.Any()).
						Return(nil, jetstream.ErrStreamNotFound),
					stream.EXPECT().
						GetMsg(gomock.Any(), uint64(1)).
						Return(nil, jetstream.ErrMsgNotFound),
				)
				s.mockJS.EXPECT().Stream(gomock.Any(), "S").Return(stream, nil).Times(3)

				_, err := s.client.StreamInfo(ctx, "S")
				s.ErrorIs(err, jetstream.ErrStreamNotFound)
				_, err = s.client.StreamRead(ctx, "S", 0, 1)
				s.ErrorIs(err, jetstream.ErrStreamNotFound)
				_, err = s.client.StreamGetMsg(ctx, "S", 1)
				s.ErrorIs(err, jetstream.ErrMsgNotFound)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *ConnectionPublicTestSuite) TestJetStream() {
	tests := []struct {
		name         string
		setup        func()
		validateFunc func(jetstream.JetStream, error)
	}{
		{
			name:  "when connected returns the JetStream API of the connection",
			setup: func() {},
			validateFunc: func(js jetstream.JetStream, err error) {
				s.NoError(err)
				s.Same(s.mockJS, js)
			},
		},
		{
			name:  "when not connected returns ErrNotConnected",
			setup: func() { client.SetJetStream(s.client, nil) },
			validateFunc: func(js jetstream.JetStream, err error) {
				s.ErrorIs(err, client.ErrNotConnected)
				s.Nil(js)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.setup()
			tc.validateFunc(s.client.JetStream())
		})
	}
}

func (s *ConnectionPublicTestSuite) TestConnectedURL() {
	tests := []struct {
		name         string
		setupFunc    func()
		validateFunc func(string)
	}{
		{
			name: "returns connected URL",
			setupFunc: func() {
				s.mockNATS.EXPECT().
					ConnectedUrl().
					Return("nats://localhost:4222")
			},
			validateFunc: func(url string) {
				s.Equal("nats://localhost:4222", url)
			},
		},
		{
			name: "returns empty string when NC is nil",
			setupFunc: func() {
				s.client.NC = nil
			},
			validateFunc: func(url string) {
				s.Empty(url)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.setupFunc()
			tc.validateFunc(s.client.ConnectedURL())
		})
	}
}

func (s *ConnectionPublicTestSuite) TestConnectedServerVersion() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the version of the connection",
			validateFunc: func() {
				s.mockNATS.EXPECT().ConnectedServerVersion().Return("2.11.0")
				s.Equal("2.11.0", s.client.ConnectedServerVersion())
			},
		},
		{
			name: "returns empty string when NC is nil",
			validateFunc: func() {
				s.client.NC = nil
				s.Empty(s.client.ConnectedServerVersion())
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func TestConnectionPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionPublicTestSuite))
}

// ConnectionServerPublicTestSuite runs calls against a real NATS server while
// the client closes and connects again, because only a real connection shows
// a race on it. Run it with -race.
type ConnectionServerPublicTestSuite struct {
	suite.Suite

	server *server.Server
	client *client.Client
}

func (s *ConnectionServerPublicTestSuite) SetupTest() {
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
	s.client = client.New(
		slog.New(slog.DiscardHandler),
		&client.Options{Host: "127.0.0.1", Port: port},
	)
	s.Require().NoError(s.client.Connect(context.Background()))
}

// syncBuffer is a bytes.Buffer that the handlers of the connection, which
// run in their own goroutine, and the test can use at the same time.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(
	p []byte,
) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// start starts a NATS server on port, or on a free port when port is -1.
func (s *ConnectionServerPublicTestSuite) start(
	port int,
) *server.Server {
	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: port})
	s.Require().NoError(err)
	srv.Start()
	s.Require().True(srv.ReadyForConnections(5 * time.Second))

	return srv
}

func (s *ConnectionServerPublicTestSuite) TearDownTest() {
	s.client.Close()
	s.server.Shutdown()
}

func (s *ConnectionServerPublicTestSuite) TestReconnect() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "a call while the client closes and connects is its result or ErrNotConnected",
			validateFunc: func() {
				ctx := context.Background()
				stop := make(chan struct{})
				var calls sync.WaitGroup
				var mu sync.Mutex
				var unexpected []error

				calls.Add(1)
				go func() {
					defer calls.Done()
					for {
						select {
						case <-stop:
							return
						default:
						}
						_ = s.client.ConnectedURL()
						_ = s.client.ConnectedServerVersion()
						for _, err := range []error{
							s.client.PublishCore(ctx, "x", []byte("y")),
							subscribe(ctx, s.client),
						} {
							if err != nil && !isClosedOrNotConnected(err) {
								mu.Lock()
								unexpected = append(unexpected, err)
								mu.Unlock()
							}
						}
					}
				}()

				for range 20 {
					s.client.Close()
					s.Require().NoError(s.client.Connect(context.Background()))
				}
				close(stop)
				calls.Wait()

				s.Empty(unexpected)
				s.NotEmpty(s.client.ConnectedURL())
			},
		},
		{
			name: "logs the drop and the reconnect when the server comes back",
			validateFunc: func() {
				srv := s.start(-1)
				port := srv.Addr().(*net.TCPAddr).Port

				var logs syncBuffer
				c := client.New(slog.New(slog.NewTextHandler(&logs, nil)), &client.Options{
					Host:      "127.0.0.1",
					Port:      port,
					Reconnect: client.ReconnectOptions{Wait: 50 * time.Millisecond},
				})
				s.Require().NoError(c.Connect(context.Background()))
				defer c.Close()

				srv.Shutdown()
				srv.WaitForShutdown()
				s.Eventually(func() bool {
					return strings.Contains(logs.String(), `msg="disconnected from NATS"`)
				}, 5*time.Second, 10*time.Millisecond)

				again := s.start(port)
				defer again.Shutdown()
				s.Eventually(func() bool {
					return strings.Contains(logs.String(), `msg="reconnected to NATS"`)
				}, 5*time.Second, 10*time.Millisecond)
				s.NotEmpty(c.ConnectedURL())
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

// subscribe subscribes to a subject and unsubscribes at once.
func subscribe(
	ctx context.Context,
	c *client.Client,
) error {
	sub, err := c.Subscribe(ctx, "x", func(context.Context, *nats.Msg) {})
	if err != nil {
		return err
	}

	return sub.Unsubscribe()
}

// isClosedOrNotConnected reports whether err is the result of a call that
// raced a Close.
func isClosedOrNotConnected(
	err error,
) bool {
	return errors.Is(err, client.ErrNotConnected) ||
		errors.Is(err, nats.ErrConnectionClosed) ||
		errors.Is(err, nats.ErrBadSubscription)
}

func (s *ConnectionServerPublicTestSuite) TestSubscribe() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "the end of ctx unsubscribes",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(context.Background())
				sub, err := s.client.Subscribe(ctx, "x.ctx", func(context.Context, *nats.Msg) {})
				s.Require().NoError(err)
				s.True(sub.IsValid())

				cancel()
				s.Eventually(
					func() bool { return !sub.IsValid() },
					2*time.Second,
					10*time.Millisecond,
				)
			},
		},
		{
			name: "while ctx is live the subscription stays",
			validateFunc: func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				sub, err := s.client.Subscribe(ctx, "x.live", func(context.Context, *nats.Msg) {})
				s.Require().NoError(err)

				s.Never(
					func() bool { return !sub.IsValid() },
					100*time.Millisecond,
					10*time.Millisecond,
				)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func TestConnectionServerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionServerPublicTestSuite))
}

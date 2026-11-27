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
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"

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
	s.client.ExtJS = s.mockJS
}

func (s *ConnectionPublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *ConnectionPublicTestSuite) TestClose() {
	tests := []struct {
		name      string
		setupFunc func()
	}{
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
		})
	}
}

func (s *ConnectionPublicTestSuite) TestCloseForgetsTheConnection() {
	s.Run(
		"a call after Close is ErrNotConnected, and a new connection asks for each handle again",
		func() {
			ctx := context.Background()
			kv := mocks.NewMockKeyValue(s.mockCtrl)
			kv.EXPECT().Get(gomock.Any(), "k").Return(nil, jetstream.ErrKeyNotFound).Times(2)
			s.mockJS.EXPECT().KeyValue(gomock.Any(), "b").Return(kv, nil)
			_, err := s.client.KVGet(ctx, "b", "k")
			s.ErrorIs(err, jetstream.ErrKeyNotFound)

			s.mockNATS.EXPECT().Close()
			s.client.Close()
			s.Nil(s.client.ExtJS)
			_, err = s.client.KVGet(ctx, "b", "k")
			s.ErrorIs(err, client.ErrNotConnected)

			js := mocks.NewMockJetStream(s.mockCtrl)
			js.EXPECT().KeyValue(gomock.Any(), "b").Return(kv, nil)
			s.client.ExtJS = js
			_, err = s.client.KVGet(ctx, "b", "k")
			s.ErrorIs(err, jetstream.ErrKeyNotFound)
		},
	)
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
	calls := map[string]func() error{
		"Publish": func() error { return s.client.Publish(ctx, "x", nil) },
		"Consume": func() error {
			return s.client.Consume(ctx, "S", "c",
				func(context.Context, jetstream.Msg) error { return nil }, nil)
		},
		"KVCreateOrUpdate": func() error {
			return s.client.KVCreateOrUpdate(ctx, jetstream.KeyValueConfig{Bucket: "b"})
		},
		"KVGet": func() error { _, err := s.client.KVGet(ctx, "b", "k"); return err },
		"ObjectStoreCreateOrUpdate": func() error {
			return s.client.ObjectStoreCreateOrUpdate(
				ctx, jetstream.ObjectStoreConfig{Bucket: "b"})
		},
		"ObjectGetBytes": func() error { _, err := s.client.ObjectGetBytes(ctx, "b", "o"); return err },
		"StreamCreateOrUpdate": func() error {
			return s.client.StreamCreateOrUpdate(ctx, jetstream.StreamConfig{Name: "S"})
		},
		"ConsumerCreateOrUpdate": func() error {
			return s.client.ConsumerCreateOrUpdate(ctx, "S", jetstream.ConsumerConfig{})
		},
		"StreamInfo": func() error { _, err := s.client.StreamInfo(ctx, "S"); return err },
	}
	for name, call := range calls {
		s.Run(name+" before Connect returns ErrNotConnected", func() {
			s.client.ExtJS = nil
			s.ErrorIs(call(), client.ErrNotConnected)
		})
	}
}

func (s *ConnectionPublicTestSuite) TestGoneHandle() {
	ctx := context.Background()

	s.Run("a bucket that is gone is asked for again at the next call", func() {
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
	})

	s.Run("an object store that is gone is asked for again at the next call", func() {
		store := mocks.NewMockObjectStore(s.mockCtrl)
		gomock.InOrder(
			store.EXPECT().GetBytes(gomock.Any(), "o").Return(nil, jetstream.ErrBucketNotFound),
			store.EXPECT().GetBytes(gomock.Any(), "o").Return([]byte("x"), nil),
		)
		s.mockJS.EXPECT().ObjectStore(gomock.Any(), "b").Return(store, nil).Times(2)

		_, err := s.client.ObjectGetBytes(ctx, "b", "o")
		s.ErrorIs(err, jetstream.ErrBucketNotFound)
		got, err := s.client.ObjectGetBytes(ctx, "b", "o")
		s.Require().NoError(err)
		s.Equal([]byte("x"), got)
	})

	s.Run("a stream that is gone is asked for again at the next call", func() {
		stream := mocks.NewMockStream(s.mockCtrl)
		gomock.InOrder(
			stream.EXPECT().Info(gomock.Any()).Return(nil, jetstream.ErrStreamNotFound),
			stream.EXPECT().
				OrderedConsumer(gomock.Any(), gomock.Any()).
				Return(nil, jetstream.ErrStreamNotFound),
			stream.EXPECT().GetMsg(gomock.Any(), uint64(1)).Return(nil, jetstream.ErrMsgNotFound),
		)
		s.mockJS.EXPECT().Stream(gomock.Any(), "S").Return(stream, nil).Times(3)

		_, err := s.client.StreamInfo(ctx, "S")
		s.ErrorIs(err, jetstream.ErrStreamNotFound)
		_, err = s.client.StreamRead(ctx, "S", 0, 1)
		s.ErrorIs(err, jetstream.ErrStreamNotFound)
		_, err = s.client.StreamGetMsg(ctx, "S", 1)
		s.ErrorIs(err, jetstream.ErrMsgNotFound)
	})
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
	s.Run("returns the version of the connection", func() {
		s.mockNATS.EXPECT().ConnectedServerVersion().Return("2.11.0")
		s.Equal("2.11.0", s.client.ConnectedServerVersion())
	})

	s.Run("returns empty string when NC is nil", func() {
		s.client.NC = nil
		s.Empty(s.client.ConnectedServerVersion())
	})
}

func TestConnectionPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionPublicTestSuite))
}

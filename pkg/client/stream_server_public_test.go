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
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
)

// StreamServerPublicTestSuite runs StreamRead against a real NATS server,
// because only a real pull request shows when a wait for messages ends.
type StreamServerPublicTestSuite struct {
	suite.Suite

	server *server.Server
	client *client.Client
}

func (s *StreamServerPublicTestSuite) SetupTest() {
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
	s.Require().NoError(s.client.Connect())

	ctx := context.Background()
	s.Require().NoError(s.client.StreamCreateOrUpdate(ctx, jetstream.StreamConfig{
		Name:     "AUDIT",
		Subjects: []string{"audit.>"},
		Storage:  jetstream.MemoryStorage,
	}))
	s.Require().NoError(s.client.Publish(ctx, "audit.x", []byte("entry-1")))
}

func (s *StreamServerPublicTestSuite) TearDownTest() {
	s.client.Close()
	s.server.Shutdown()
}

func (s *StreamServerPublicTestSuite) TestStreamRead() {
	tests := []struct {
		name         string
		ctx          func() (context.Context, context.CancelFunc)
		validateFunc func(msgs []jetstream.Msg, err error, took time.Duration)
	}{
		{
			name: "with fewer messages than count returns them at once",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			validateFunc: func(msgs []jetstream.Msg, err error, took time.Duration) {
				s.Require().NoError(err)
				s.Require().Len(msgs, 1)
				s.Equal("entry-1", string(msgs[0].Data()))
				s.Less(took, time.Second)
			},
		},
		{
			name: "when ctx already ended returns its error",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx, cancel
			},
			validateFunc: func(_ []jetstream.Msg, err error, _ time.Duration) {
				s.ErrorIs(err, context.Canceled)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			ctx, cancel := tc.ctx()
			defer cancel()

			began := time.Now()
			msgs, err := s.client.StreamRead(ctx, "AUDIT", 0, 3)
			tc.validateFunc(msgs, err, time.Since(began))
		})
	}
}

func (s *StreamServerPublicTestSuite) TestStreamReadLeavesNoConsumer() {
	ctx := context.Background()
	for range 3 {
		_, err := s.client.StreamRead(ctx, "AUDIT", 0, 3)
		s.Require().NoError(err)
	}

	info, err := s.client.StreamInfo(ctx, "AUDIT")
	s.Require().NoError(err)
	s.Equal(0, info.State.Consumers)
}

func TestStreamServerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(StreamServerPublicTestSuite))
}

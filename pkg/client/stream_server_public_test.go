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
	s.Run("with fewer messages than count returns them at the deadline of ctx", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()

		msgs, err := s.client.StreamRead(ctx, "AUDIT", 0, 3)
		s.Require().NoError(err)
		s.Require().Len(msgs, 1)
		s.Equal("entry-1", string(msgs[0].Data()))
	})

	s.Run("when ctx ends with no deadline returns at once", func() {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(100*time.Millisecond, cancel)

		began := time.Now()
		msgs, err := s.client.StreamRead(ctx, "AUDIT", 0, 3)
		s.ErrorIs(err, context.Canceled)
		s.Len(msgs, 1)
		s.Less(time.Since(began), 2*time.Second)
	})

	s.Run("when ctx already ended returns its error", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := s.client.StreamRead(ctx, "AUDIT", 0, 3)
		s.ErrorIs(err, context.Canceled)
	})
}

func TestStreamServerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(StreamServerPublicTestSuite))
}

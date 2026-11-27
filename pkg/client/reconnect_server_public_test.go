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
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
)

// ReconnectServerPublicTestSuite stops a real NATS server under a connected
// client and starts it again, because only a real connection shows the
// reconnect and its log.
type ReconnectServerPublicTestSuite struct {
	suite.Suite
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
func (s *ReconnectServerPublicTestSuite) start(
	port int,
) *server.Server {
	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: port})
	s.Require().NoError(err)
	srv.Start()
	s.Require().True(srv.ReadyForConnections(5 * time.Second))

	return srv
}

func (s *ReconnectServerPublicTestSuite) TestReconnect() {
	s.Run("logs the drop and the reconnect when the server comes back", func() {
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
	})
}

func TestReconnectServerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ReconnectServerPublicTestSuite))
}

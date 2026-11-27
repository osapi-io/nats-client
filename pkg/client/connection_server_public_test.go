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
	"net"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
)

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

func (s *ConnectionServerPublicTestSuite) TearDownTest() {
	s.client.Close()
	s.server.Shutdown()
}

func (s *ConnectionServerPublicTestSuite) TestCallsDuringReconnect() {
	s.Run("a call while the client closes and connects is its result or ErrNotConnected", func() {
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
	})
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

func (s *ConnectionServerPublicTestSuite) TestSubscribeEndsWithCtx() {
	s.Run("the end of ctx unsubscribes", func() {
		ctx, cancel := context.WithCancel(context.Background())
		sub, err := s.client.Subscribe(ctx, "x.ctx", func(context.Context, *nats.Msg) {})
		s.Require().NoError(err)
		s.True(sub.IsValid())

		cancel()
		s.Eventually(func() bool { return !sub.IsValid() }, 2*time.Second, 10*time.Millisecond)
	})
}

func TestConnectionServerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionServerPublicTestSuite))
}

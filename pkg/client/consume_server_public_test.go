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
	"net"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
)

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
	s.Run("gives a published message to the handler", func() {
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

		s.Require().NoError(s.client.Publish(context.Background(), "jobs.x", []byte("job-1")))
		select {
		case data := <-got:
			s.Equal("job-1", data)
		case <-time.After(5 * time.Second):
			s.Fail("the handler did not get the message")
		}
		cancel()
		s.ErrorIs(<-done, context.Canceled)
	})

	s.Run("handles up to MaxInFlight messages at the same time", func() {
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
	})

	s.Run("when ctx ends while it waits for a message returns at once", func() {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- s.client.Consume(ctx, "JOBS", "agent",
				func(context.Context, jetstream.Msg) error { return nil }, nil)
		}()

		// Let Consume start its wait. The pull request expires after 30
		// seconds, so a return within 2 seconds shows that ctx ended it.
		time.Sleep(200 * time.Millisecond)
		began := time.Now()
		cancel()

		select {
		case err := <-done:
			s.ErrorIs(err, context.Canceled)
			s.Less(time.Since(began), 2*time.Second)
		case <-time.After(5 * time.Second):
			s.Fail("Consume did not return when ctx ended")
		}
	})
}

func TestConsumeServerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConsumeServerPublicTestSuite))
}

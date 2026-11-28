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
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type StreamPublicTestSuite struct {
	suite.Suite

	ctrl   *gomock.Controller
	js     *mocks.MockJetStream
	stream *mocks.MockStream
	client *client.Client
	ctx    context.Context
}

func (s *StreamPublicTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.js = mocks.NewMockJetStream(s.ctrl)
	s.stream = mocks.NewMockStream(s.ctrl)
	s.client = client.New(slog.Default(), &client.Options{})
	client.SetJetStream(s.client, s.js)
	s.ctx = context.Background()
}

func (s *StreamPublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

// open makes the server return the handle of the stream "S" once.
func (s *StreamPublicTestSuite) open() {
	s.js.EXPECT().Stream(gomock.Any(), "S").Return(s.stream, nil)
}

func (s *StreamPublicTestSuite) TestStreamCreateOrUpdate() {
	cfg := jetstream.StreamConfig{Name: "S", Subjects: []string{"jobs.>"}}

	tests := []struct {
		name     string
		setup    func()
		wantErr  string
		wantKept bool
		// server, when set, runs the call against a real NATS server,
		// because only a server shows which storage type it keeps.
		server       *jetstream.StreamConfig
		validateFunc func(jetstream.StreamConfig)
	}{
		{
			name: "when the stream is made keeps its handle",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateStream(gomock.Any(), cfg).Return(s.stream, nil)
			},
			wantKept: true,
		},
		{
			name: "when the storage type differs keeps it and applies the rest",
			setup: func() {
				kept := cfg
				kept.Storage = jetstream.MemoryStorage
				s.js.EXPECT().CreateOrUpdateStream(gomock.Any(), cfg).Return(nil, errStorageType)
				s.js.EXPECT().Stream(gomock.Any(), "S").Return(s.stream, nil)
				s.stream.EXPECT().CachedInfo().Return(&jetstream.StreamInfo{
					Config: jetstream.StreamConfig{Name: "S", Storage: jetstream.MemoryStorage},
				})
				s.js.EXPECT().UpdateStream(gomock.Any(), kept).Return(s.stream, nil)
			},
			wantKept: true,
		},
		{
			name: "when the storage type differs and the stream cannot be read returns the error",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateStream(gomock.Any(), cfg).Return(nil, errStorageType)
				s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, errBoom)
			},
			wantErr: "error creating/updating stream S: boom",
		},
		{
			name: "when the storage type differs and the update fails returns the error",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateStream(gomock.Any(), cfg).Return(nil, errStorageType)
				s.js.EXPECT().Stream(gomock.Any(), "S").Return(s.stream, nil)
				s.stream.EXPECT().CachedInfo().Return(&jetstream.StreamInfo{})
				s.js.EXPECT().UpdateStream(gomock.Any(), cfg).Return(nil, errBoom)
			},
			wantErr: "error creating/updating stream S: boom",
		},
		{
			name: "when the server refuses returns the error",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateStream(gomock.Any(), cfg).Return(nil, errBoom)
			},
			wantErr: "error creating/updating stream S: boom",
		},
		{
			name: "on a server a new storage type keeps the old one, and applies the rest",
			server: &jetstream.StreamConfig{
				Name:     "AUDIT",
				Subjects: []string{"audit.>", "more.>"},
				Storage:  jetstream.FileStorage,
			},
			validateFunc: func(got jetstream.StreamConfig) {
				s.Equal(jetstream.MemoryStorage, got.Storage)
				s.Equal([]string{"audit.>", "more.>"}, got.Subjects)
			},
		},
		{
			name: "on a server the same storage type applies the change",
			server: &jetstream.StreamConfig{
				Name:     "AUDIT",
				Subjects: []string{"audit.>", "other.>"},
				Storage:  jetstream.MemoryStorage,
			},
			validateFunc: func(got jetstream.StreamConfig) {
				s.Equal(jetstream.MemoryStorage, got.Storage)
				s.Equal([]string{"audit.>", "other.>"}, got.Subjects)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			if tc.server != nil {
				c := connectJetStream(s.T())
				s.Require().NoError(c.StreamCreateOrUpdate(s.ctx, jetstream.StreamConfig{
					Name:     "AUDIT",
					Subjects: []string{"audit.>"},
					Storage:  jetstream.MemoryStorage,
				}))
				s.Require().NoError(c.StreamCreateOrUpdate(s.ctx, *tc.server))

				info, err := c.StreamInfo(s.ctx, "AUDIT")
				s.Require().NoError(err)
				tc.validateFunc(info.Config)

				return
			}
			tc.setup()

			err := s.client.StreamCreateOrUpdate(s.ctx, cfg)
			if tc.wantErr != "" {
				s.EqualError(err, tc.wantErr)
				return
			}
			s.Require().NoError(err)

			if !tc.wantKept {
				s.open()
			}
			s.stream.EXPECT().Info(gomock.Any()).Return(&jetstream.StreamInfo{}, nil)
			_, err = s.client.StreamInfo(s.ctx, "S")
			s.NoError(err)
		})
	}
}

func (s *StreamPublicTestSuite) TestConsumerCreateOrUpdate() {
	cfg := jetstream.ConsumerConfig{Durable: "c"}

	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "makes the consumer",
			validateFunc: func() {
				s.js.EXPECT().CreateOrUpdateConsumer(gomock.Any(), "S", cfg).Return(nil, nil)

				s.NoError(s.client.ConsumerCreateOrUpdate(s.ctx, "S", cfg))
			},
		},
		{
			name: "when the server refuses returns the error",
			validateFunc: func() {
				s.js.EXPECT().CreateOrUpdateConsumer(gomock.Any(), "S", cfg).Return(nil, errBoom)

				s.EqualError(
					s.client.ConsumerCreateOrUpdate(s.ctx, "S", cfg),
					"error creating consumer for stream S: boom",
				)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *StreamPublicTestSuite) TestStreamInfo() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the info and asks for the handle once",
			validateFunc: func() {
				info := &jetstream.StreamInfo{State: jetstream.StreamState{Msgs: 3}}
				s.open()
				s.stream.EXPECT().Info(gomock.Any()).Return(info, nil).Times(2)

				got, err := s.client.StreamInfo(s.ctx, "S")
				s.NoError(err)
				s.Equal(info, got)
				_, err = s.client.StreamInfo(s.ctx, "S")
				s.NoError(err)
			},
		},
		{
			name: "when the stream is not there returns the error",
			validateFunc: func() {
				s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, jetstream.ErrStreamNotFound)

				_, err := s.client.StreamInfo(s.ctx, "S")
				s.ErrorIs(err, jetstream.ErrStreamNotFound)
				s.ErrorContains(err, "failed to get stream S")
			},
		},
		{
			name: "when the info fails returns the error",
			validateFunc: func() {
				s.open()
				s.stream.EXPECT().Info(gomock.Any()).Return(nil, errBoom)

				_, err := s.client.StreamInfo(s.ctx, "S")
				s.EqualError(err, "failed to get stream info for S: boom")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *StreamPublicTestSuite) TestStreamLastMsg() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the last message on the subject",
			validateFunc: func() {
				msg := &jetstream.RawStreamMsg{Subject: "audit.x", Sequence: 9}
				s.open()
				s.stream.EXPECT().GetLastMsgForSubject(gomock.Any(), "audit.x").Return(msg, nil)

				got, err := s.client.StreamLastMsg(s.ctx, "S", "audit.x")
				s.NoError(err)
				s.Equal(msg, got)
			},
		},
		{
			name: "when the stream is not there returns the error",
			validateFunc: func() {
				s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, errBoom)

				_, err := s.client.StreamLastMsg(s.ctx, "S", "audit.x")
				s.ErrorIs(err, errBoom)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *StreamPublicTestSuite) TestStreamGetMsg() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the message at the sequence",
			validateFunc: func() {
				msg := &jetstream.RawStreamMsg{Subject: "jobs.x", Sequence: 7}
				s.open()
				s.stream.EXPECT().GetMsg(gomock.Any(), uint64(7)).Return(msg, nil)

				got, err := s.client.StreamGetMsg(s.ctx, "S", 7)
				s.NoError(err)
				s.Equal(msg, got)
			},
		},
		{
			name: "when the stream does not have it returns the error",
			validateFunc: func() {
				s.open()
				s.stream.EXPECT().
					GetMsg(gomock.Any(), uint64(7)).
					Return(nil, jetstream.ErrMsgNotFound)

				_, err := s.client.StreamGetMsg(s.ctx, "S", 7)
				s.ErrorIs(err, jetstream.ErrMsgNotFound)
			},
		},
		{
			name: "when the stream is not there returns the error",
			validateFunc: func() {
				s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, errBoom)

				_, err := s.client.StreamGetMsg(s.ctx, "S", 7)
				s.ErrorIs(err, errBoom)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *StreamPublicTestSuite) TestStreamPurgeSubject() {
	s.Run("purges the messages of the subject", func() {
		s.open()
		s.stream.EXPECT().Purge(gomock.Any(), gomock.Any()).Return(nil)

		s.NoError(s.client.StreamPurgeSubject(s.ctx, "S", "$KV.jobs.jobs.1"))
	})

	s.Run("when the purge fails returns the error with the subject", func() {
		s.open()
		s.stream.EXPECT().Purge(gomock.Any(), gomock.Any()).Return(errBoom)

		err := s.client.StreamPurgeSubject(s.ctx, "S", "$KV.jobs.jobs.1")
		s.ErrorIs(err, errBoom)
		s.ErrorContains(err, "failed to purge $KV.jobs.jobs.1 from stream S")
	})

	s.Run("when the stream is not there returns the error", func() {
		s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, errBoom)

		s.ErrorIs(s.client.StreamPurgeSubject(s.ctx, "S", "$KV.jobs.jobs.1"), errBoom)
	})
}

// batch returns a batch of msgs that ends with err.
func (s *StreamPublicTestSuite) batch(
	err error,
	msgs ...jetstream.Msg,
) *mocks.MockMessageBatch {
	ch := make(chan jetstream.Msg, len(msgs))
	for _, m := range msgs {
		ch <- m
	}
	close(ch)
	b := mocks.NewMockMessageBatch(s.ctrl)
	b.EXPECT().Messages().Return(ch)
	b.EXPECT().Error().Return(err)

	return b
}

func (s *StreamPublicTestSuite) TestStreamRead() {
	first := mocks.NewMockMsg(gomock.NewController(s.T()))
	second := mocks.NewMockMsg(gomock.NewController(s.T()))
	made := &jetstream.ConsumerInfo{Stream: "S", Name: "ordered-1"}
	// ordered sets up the stream "S" to make an ordered consumer from cfg,
	// and lets the row set what the consumer does.
	ordered := func(
		cfg jetstream.OrderedConsumerConfig,
		consumer func(*mocks.MockConsumer),
	) func() {
		return func() {
			c := mocks.NewMockConsumer(s.ctrl)
			s.open()
			s.stream.EXPECT().OrderedConsumer(gomock.Any(), cfg).Return(c, nil)
			consumer(c)
		}
	}
	all := jetstream.OrderedConsumerConfig{DeliverPolicy: jetstream.DeliverAllPolicy}

	tests := []struct {
		name  string
		setup func()
		from  uint64
		count int
		// server, when set, runs the call against a real NATS server that
		// holds one message in "AUDIT", because only a real pull request
		// shows when a wait for messages ends.
		server       bool
		ctx          func() (context.Context, context.CancelFunc)
		validateFunc func(c *client.Client, msgs []jetstream.Msg, err error, took time.Duration)
	}{
		{
			name:  "from 0 reads from the first message, and deletes its consumer",
			count: 2,
			setup: ordered(all, func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(s.batch(nil, first, second), nil)
				c.EXPECT().CachedInfo().Return(made)
				s.stream.EXPECT().DeleteConsumer(gomock.Any(), "ordered-1").Return(nil)
			}),
			validateFunc: func(_ *client.Client, got []jetstream.Msg, err error, _ time.Duration) {
				s.NoError(err)
				s.Equal([]jetstream.Msg{first, second}, got)
			},
		},
		{
			name:  "from a sequence reads from that sequence",
			count: 2,
			from:  7,
			setup: ordered(jetstream.OrderedConsumerConfig{
				DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
				OptStartSeq:   7,
			}, func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(s.batch(nil, first), nil)
				c.EXPECT().CachedInfo().Return(made)
				s.stream.EXPECT().DeleteConsumer(gomock.Any(), "ordered-1").Return(nil)
			}),
			validateFunc: func(_ *client.Client, got []jetstream.Msg, err error, _ time.Duration) {
				s.NoError(err)
				s.Equal([]jetstream.Msg{first}, got)
			},
		},
		{
			name:  "when the delete of its consumer fails still returns the messages",
			count: 2,
			setup: ordered(all, func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(s.batch(nil, first), nil)
				c.EXPECT().CachedInfo().Return(made)
				s.stream.EXPECT().DeleteConsumer(gomock.Any(), "ordered-1").Return(errBoom)
			}),
			validateFunc: func(_ *client.Client, got []jetstream.Msg, err error, _ time.Duration) {
				s.NoError(err)
				s.Equal([]jetstream.Msg{first}, got)
			},
		},
		{
			name:  "when the fetch fails returns the error, and deletes no consumer that was never made",
			count: 2,
			setup: ordered(all, func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(nil, errBoom)
				c.EXPECT().CachedInfo().Return(nil)
			}),
			validateFunc: func(_ *client.Client, _ []jetstream.Msg, err error, _ time.Duration) {
				s.EqualError(err, "read stream S: boom")
			},
		},
		{
			name:  "when the batch ends with an error returns the messages and the error",
			count: 2,
			setup: ordered(all, func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(s.batch(errBoom, first), nil)
				c.EXPECT().CachedInfo().Return(&jetstream.ConsumerInfo{})
			}),
			validateFunc: func(_ *client.Client, got []jetstream.Msg, err error, _ time.Duration) {
				s.EqualError(err, "read stream S: boom")
				s.Equal([]jetstream.Msg{first}, got)
			},
		},
		{
			name:  "when the stream is not there returns the error",
			count: 2,
			setup: func() {
				s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, errBoom)
			},
			validateFunc: func(_ *client.Client, _ []jetstream.Msg, err error, _ time.Duration) {
				s.ErrorIs(err, errBoom)
			},
		},
		{
			name:  "when the consumer cannot be made returns the error",
			count: 2,
			setup: func() {
				s.open()
				s.stream.EXPECT().OrderedConsumer(gomock.Any(), gomock.Any()).Return(nil, errBoom)
			},
			validateFunc: func(_ *client.Client, _ []jetstream.Msg, err error, _ time.Duration) {
				s.EqualError(err, "create ordered consumer on S: boom")
			},
		},
		{
			name:  "with a count of 0 returns an error",
			count: 0,
			validateFunc: func(_ *client.Client, _ []jetstream.Msg, err error, _ time.Duration) {
				s.EqualError(err, "read stream S: count 0 is less than 1")
			},
		},
		{
			name:  "with a count of -1 returns an error",
			count: -1,
			validateFunc: func(_ *client.Client, _ []jetstream.Msg, err error, _ time.Duration) {
				s.EqualError(err, "read stream S: count -1 is less than 1")
			},
		},
		{
			name:  "with a count one more than MaxStreamReadCount returns an error",
			count: client.MaxStreamReadCount + 1,
			validateFunc: func(_ *client.Client, _ []jetstream.Msg, err error, _ time.Duration) {
				s.EqualError(err, fmt.Sprintf(
					"read stream S: count %d is more than %d",
					client.MaxStreamReadCount+1,
					client.MaxStreamReadCount,
				))
			},
		},
		{
			name:  "with a huge count returns an error, and allocates nothing",
			count: 1 << 40,
			validateFunc: func(_ *client.Client, _ []jetstream.Msg, err error, _ time.Duration) {
				s.EqualError(err, fmt.Sprintf(
					"read stream S: count %d is more than %d",
					1<<40,
					client.MaxStreamReadCount,
				))
			},
		},
		{
			name:   "on a server with fewer messages than count returns them at once",
			count:  3,
			server: true,
			validateFunc: func(_ *client.Client, msgs []jetstream.Msg, err error, took time.Duration) {
				s.Require().NoError(err)
				s.Require().Len(msgs, 1)
				s.Equal("entry-1", string(msgs[0].Data()))
				s.Less(took, time.Second)
			},
		},
		{
			name:   "on a server when ctx already ended returns its error",
			count:  3,
			server: true,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx, cancel
			},
			validateFunc: func(_ *client.Client, _ []jetstream.Msg, err error, _ time.Duration) {
				s.ErrorIs(err, context.Canceled)
			},
		},
		{
			name:   "on a server leaves no consumer behind",
			count:  3,
			server: true,
			validateFunc: func(c *client.Client, _ []jetstream.Msg, err error, _ time.Duration) {
				s.Require().NoError(err)
				for range 2 {
					_, err := c.StreamRead(context.Background(), "AUDIT", 0, 3)
					s.Require().NoError(err)
				}

				info, err := c.StreamInfo(context.Background(), "AUDIT")
				s.Require().NoError(err)
				s.Equal(0, info.State.Consumers)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			ctx, cancel := context.WithCancel(s.ctx)
			if tc.ctx != nil {
				ctx, cancel = tc.ctx()
			}
			defer cancel()

			c, stream := s.client, "S"
			if tc.server {
				c, stream = connectJetStream(s.T()), "AUDIT"
				s.Require().NoError(c.StreamCreateOrUpdate(s.ctx, jetstream.StreamConfig{
					Name:     "AUDIT",
					Subjects: []string{"audit.>"},
					Storage:  jetstream.MemoryStorage,
				}))
				s.Require().NoError(c.Publish(s.ctx, "audit.x", []byte("entry-1")))
			}
			if tc.setup != nil {
				tc.setup()
			}

			began := time.Now()
			msgs, err := c.StreamRead(ctx, stream, tc.from, tc.count)
			tc.validateFunc(c, msgs, err, time.Since(began))
		})
	}
}

func TestStreamPublicTestSuite(t *testing.T) {
	suite.Run(t, new(StreamPublicTestSuite))
}

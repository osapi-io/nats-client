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
	s.client.ExtJS = s.js
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
	}{
		{
			name: "when the stream is made keeps its handle",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateStream(gomock.Any(), cfg).Return(s.stream, nil)
			},
			wantKept: true,
		},
		{
			name: "when only the storage type differs keeps the stream",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateStream(gomock.Any(), cfg).Return(nil, errStorageType)
			},
		},
		{
			name: "when the server refuses returns the error",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateStream(gomock.Any(), cfg).Return(nil, errBoom)
			},
			wantErr: "error creating/updating stream S: boom",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
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

	s.Run("makes the consumer", func() {
		s.js.EXPECT().CreateOrUpdateConsumer(gomock.Any(), "S", cfg).Return(nil, nil)

		s.NoError(s.client.ConsumerCreateOrUpdate(s.ctx, "S", cfg))
	})

	s.Run("when the server refuses returns the error", func() {
		s.js.EXPECT().CreateOrUpdateConsumer(gomock.Any(), "S", cfg).Return(nil, errBoom)

		s.EqualError(
			s.client.ConsumerCreateOrUpdate(s.ctx, "S", cfg),
			"error creating consumer for stream S: boom",
		)
	})
}

func (s *StreamPublicTestSuite) TestStreamInfo() {
	s.Run("returns the info and asks for the handle once", func() {
		info := &jetstream.StreamInfo{State: jetstream.StreamState{Msgs: 3}}
		s.open()
		s.stream.EXPECT().Info(gomock.Any()).Return(info, nil).Times(2)

		got, err := s.client.StreamInfo(s.ctx, "S")
		s.NoError(err)
		s.Equal(info, got)
		_, err = s.client.StreamInfo(s.ctx, "S")
		s.NoError(err)
	})

	s.Run("when the stream is not there returns the error", func() {
		s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, jetstream.ErrStreamNotFound)

		_, err := s.client.StreamInfo(s.ctx, "S")
		s.ErrorIs(err, jetstream.ErrStreamNotFound)
		s.ErrorContains(err, "failed to get stream S")
	})

	s.Run("when the info fails returns the error", func() {
		s.open()
		s.stream.EXPECT().Info(gomock.Any()).Return(nil, errBoom)

		_, err := s.client.StreamInfo(s.ctx, "S")
		s.EqualError(err, "failed to get stream info for S: boom")
	})
}

func (s *StreamPublicTestSuite) TestStreamLastMsg() {
	s.Run("returns the last message on the subject", func() {
		msg := &jetstream.RawStreamMsg{Subject: "audit.x", Sequence: 9}
		s.open()
		s.stream.EXPECT().GetLastMsgForSubject(gomock.Any(), "audit.x").Return(msg, nil)

		got, err := s.client.StreamLastMsg(s.ctx, "S", "audit.x")
		s.NoError(err)
		s.Equal(msg, got)
	})

	s.Run("when the stream is not there returns the error", func() {
		s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, errBoom)

		_, err := s.client.StreamLastMsg(s.ctx, "S", "audit.x")
		s.ErrorIs(err, errBoom)
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

	tests := []struct {
		name    string
		ctx     func() (context.Context, context.CancelFunc)
		from    uint64
		setup   func(*mocks.MockConsumer)
		wantCfg jetstream.OrderedConsumerConfig
		want    []jetstream.Msg
		wantErr string
	}{
		{
			name: "from 0 reads from the first message",
			setup: func(c *mocks.MockConsumer) {
				c.EXPECT().Fetch(2, gomock.Any()).Return(s.batch(nil, first, second), nil)
			},
			wantCfg: jetstream.OrderedConsumerConfig{DeliverPolicy: jetstream.DeliverAllPolicy},
			want:    []jetstream.Msg{first, second},
		},
		{
			name: "from a sequence reads from that sequence, until the deadline of ctx",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Second)
			},
			from: 7,
			setup: func(c *mocks.MockConsumer) {
				c.EXPECT().Fetch(2, gomock.Any()).Return(s.batch(nil, first), nil)
			},
			wantCfg: jetstream.OrderedConsumerConfig{
				DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
				OptStartSeq:   7,
			},
			want: []jetstream.Msg{first},
		},
		{
			name: "when the fetch fails returns the error",
			setup: func(c *mocks.MockConsumer) {
				c.EXPECT().Fetch(2, gomock.Any()).Return(nil, errBoom)
			},
			wantCfg: jetstream.OrderedConsumerConfig{DeliverPolicy: jetstream.DeliverAllPolicy},
			wantErr: "read stream S: boom",
		},
		{
			name: "when the batch ends with an error returns the messages and the error",
			setup: func(c *mocks.MockConsumer) {
				c.EXPECT().Fetch(2, gomock.Any()).Return(s.batch(errBoom, first), nil)
			},
			wantCfg: jetstream.OrderedConsumerConfig{DeliverPolicy: jetstream.DeliverAllPolicy},
			want:    []jetstream.Msg{first},
			wantErr: "read stream S: boom",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			ctx, cancel := context.WithCancel(s.ctx)
			if tc.ctx != nil {
				ctx, cancel = tc.ctx()
			}
			defer cancel()

			consumer := mocks.NewMockConsumer(s.ctrl)
			s.open()
			s.stream.EXPECT().OrderedConsumer(gomock.Any(), tc.wantCfg).Return(consumer, nil)
			tc.setup(consumer)

			got, err := s.client.StreamRead(ctx, "S", tc.from, 2)
			if tc.wantErr != "" {
				s.EqualError(err, tc.wantErr)
			} else {
				s.NoError(err)
			}
			if tc.want != nil {
				s.Equal(tc.want, got)
			}
		})
	}

	s.Run("when the stream is not there returns the error", func() {
		s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, errBoom)

		_, err := s.client.StreamRead(s.ctx, "S", 0, 2)
		s.ErrorIs(err, errBoom)
	})

	s.Run("when the consumer cannot be made returns the error", func() {
		s.open()
		s.stream.EXPECT().OrderedConsumer(gomock.Any(), gomock.Any()).Return(nil, errBoom)

		_, err := s.client.StreamRead(s.ctx, "S", 0, 2)
		s.EqualError(err, "create ordered consumer on S: boom")
	})
}

func TestStreamPublicTestSuite(t *testing.T) {
	suite.Run(t, new(StreamPublicTestSuite))
}

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
	"testing"

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

func (s *StreamPublicTestSuite) TestStreamGetMsg() {
	s.Run("returns the message at the sequence", func() {
		msg := &jetstream.RawStreamMsg{Subject: "jobs.x", Sequence: 7}
		s.open()
		s.stream.EXPECT().GetMsg(gomock.Any(), uint64(7)).Return(msg, nil)

		got, err := s.client.StreamGetMsg(s.ctx, "S", 7)
		s.NoError(err)
		s.Equal(msg, got)
	})

	s.Run("when the stream does not have it returns the error", func() {
		s.open()
		s.stream.EXPECT().GetMsg(gomock.Any(), uint64(7)).Return(nil, jetstream.ErrMsgNotFound)

		_, err := s.client.StreamGetMsg(s.ctx, "S", 7)
		s.ErrorIs(err, jetstream.ErrMsgNotFound)
	})

	s.Run("when the stream is not there returns the error", func() {
		s.js.EXPECT().Stream(gomock.Any(), "S").Return(nil, errBoom)

		_, err := s.client.StreamGetMsg(s.ctx, "S", 7)
		s.ErrorIs(err, errBoom)
	})
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

	tests := []struct {
		name    string
		from    uint64
		setup   func(*mocks.MockConsumer)
		wantCfg jetstream.OrderedConsumerConfig
		want    []jetstream.Msg
		wantErr string
	}{
		{
			name: "from 0 reads from the first message, and deletes its consumer",
			setup: func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(s.batch(nil, first, second), nil)
				c.EXPECT().CachedInfo().Return(made)
				s.stream.EXPECT().DeleteConsumer(gomock.Any(), "ordered-1").Return(nil)
			},
			wantCfg: jetstream.OrderedConsumerConfig{DeliverPolicy: jetstream.DeliverAllPolicy},
			want:    []jetstream.Msg{first, second},
		},
		{
			name: "from a sequence reads from that sequence",
			from: 7,
			setup: func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(s.batch(nil, first), nil)
				c.EXPECT().CachedInfo().Return(made)
				s.stream.EXPECT().DeleteConsumer(gomock.Any(), "ordered-1").Return(nil)
			},
			wantCfg: jetstream.OrderedConsumerConfig{
				DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
				OptStartSeq:   7,
			},
			want: []jetstream.Msg{first},
		},
		{
			name: "when the delete of its consumer fails still returns the messages",
			setup: func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(s.batch(nil, first), nil)
				c.EXPECT().CachedInfo().Return(made)
				s.stream.EXPECT().DeleteConsumer(gomock.Any(), "ordered-1").Return(errBoom)
			},
			wantCfg: jetstream.OrderedConsumerConfig{DeliverPolicy: jetstream.DeliverAllPolicy},
			want:    []jetstream.Msg{first},
		},
		{
			name: "when the fetch fails returns the error, and deletes no consumer that was never made",
			setup: func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(nil, errBoom)
				c.EXPECT().CachedInfo().Return(nil)
			},
			wantCfg: jetstream.OrderedConsumerConfig{DeliverPolicy: jetstream.DeliverAllPolicy},
			wantErr: "read stream S: boom",
		},
		{
			name: "when the batch ends with an error returns the messages and the error",
			setup: func(c *mocks.MockConsumer) {
				c.EXPECT().FetchNoWait(2).Return(s.batch(errBoom, first), nil)
				c.EXPECT().CachedInfo().Return(&jetstream.ConsumerInfo{})
			},
			wantCfg: jetstream.OrderedConsumerConfig{DeliverPolicy: jetstream.DeliverAllPolicy},
			want:    []jetstream.Msg{first},
			wantErr: "read stream S: boom",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			consumer := mocks.NewMockConsumer(s.ctrl)
			s.open()
			s.stream.EXPECT().OrderedConsumer(gomock.Any(), tc.wantCfg).Return(consumer, nil)
			tc.setup(consumer)

			got, err := s.client.StreamRead(s.ctx, "S", tc.from, 2)
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

	s.Run("with a count less than 1 returns an error", func() {
		for _, count := range []int{0, -1} {
			_, err := s.client.StreamRead(s.ctx, "S", 0, count)
			s.EqualError(err, fmt.Sprintf("read stream S: count %d is less than 1", count))
		}
	})
}

func TestStreamPublicTestSuite(t *testing.T) {
	suite.Run(t, new(StreamPublicTestSuite))
}

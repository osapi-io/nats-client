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
	"errors"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type JetStreamPublicTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockExt  *mocks.MockJetStream
	client   *client.Client
	ctx      context.Context
}

func (s *JetStreamPublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockExt = mocks.NewMockJetStream(s.mockCtrl)
	s.client = client.New(slog.Default(), &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	})
	s.client.ExtJS = s.mockExt
	s.ctx = context.Background()
}

func (s *JetStreamPublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *JetStreamPublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *JetStreamPublicTestSuite) TestCreateOrUpdateStreamWithConfig() {
	tests := []struct {
		name         string
		config       jetstream.StreamConfig
		mockSetup    func()
		validateFunc func(error)
	}{
		{
			name:   "successfully creates stream",
			config: jetstream.StreamConfig{Name: "test-stream", Subjects: []string{"test.*"}},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateStream(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:   "error creating stream",
			config: jetstream.StreamConfig{Name: "test-stream"},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateStream(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("stream creation failed")).
					Times(1)
			},
			validateFunc: func(err error) {
				s.EqualError(
					err,
					"error creating/updating stream test-stream: stream creation failed",
				)
			},
		},
		{
			name: "when storage type conflict returns success",
			config: jetstream.StreamConfig{
				Name:    "test-stream",
				Storage: jetstream.MemoryStorage,
			},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateStream(gomock.Any(), gomock.Any()).
					Return(nil, &jetstream.APIError{Code: 500, ErrorCode: 10052, Description: "stream configuration update can not change storage type"}).
					Times(1)
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			tc.validateFunc(s.client.CreateOrUpdateStreamWithConfig(s.ctx, tc.config))
		})
	}
}

func (s *JetStreamPublicTestSuite) TestCreateOrUpdateConsumerWithConfig() {
	tests := []struct {
		name         string
		streamName   string
		config       jetstream.ConsumerConfig
		mockSetup    func()
		validateFunc func(error)
	}{
		{
			name:       "successfully creates consumer",
			streamName: "test-stream",
			config:     jetstream.ConsumerConfig{Durable: "consumer-1"},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateConsumer(gomock.Any(), "test-stream", gomock.Any()).
					Return(nil, nil).
					Times(1)
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:       "error creating consumer",
			streamName: "test-stream",
			config:     jetstream.ConsumerConfig{Durable: "consumer-1"},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateConsumer(gomock.Any(), "test-stream", gomock.Any()).
					Return(nil, errors.New("consumer creation failed")).
					Times(1)
			},
			validateFunc: func(err error) {
				s.EqualError(
					err,
					"error creating consumer for stream test-stream: consumer creation failed",
				)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			tc.validateFunc(
				s.client.CreateOrUpdateConsumerWithConfig(s.ctx, tc.streamName, tc.config),
			)
		})
	}
}

func (s *JetStreamPublicTestSuite) TestCreateOrUpdateJetStreamWithConfig() {
	tests := []struct {
		name            string
		streamConfig    jetstream.StreamConfig
		consumerConfigs []jetstream.ConsumerConfig
		mockSetup       func()
		validateFunc    func(error)
	}{
		{
			name:         "successfully creates stream and consumers",
			streamConfig: jetstream.StreamConfig{Name: "test-stream", Subjects: []string{"test.*"}},
			consumerConfigs: []jetstream.ConsumerConfig{
				{Durable: "consumer-1"},
				{Durable: "consumer-2"},
			},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateStream(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)
				s.mockExt.EXPECT().
					CreateOrUpdateConsumer(gomock.Any(), "test-stream", gomock.Any()).
					Return(nil, nil).
					Times(2)
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:            "error creating stream",
			streamConfig:    jetstream.StreamConfig{Name: "test-stream"},
			consumerConfigs: []jetstream.ConsumerConfig{{Durable: "consumer-1"}},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateStream(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("stream creation failed")).
					Times(1)
				s.mockExt.EXPECT().
					CreateOrUpdateConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			validateFunc: func(err error) {
				s.EqualError(
					err,
					"error creating/updating stream test-stream: stream creation failed",
				)
			},
		},
		{
			name:            "error creating consumer",
			streamConfig:    jetstream.StreamConfig{Name: "test-stream"},
			consumerConfigs: []jetstream.ConsumerConfig{{Durable: "consumer-1"}},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateStream(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)
				s.mockExt.EXPECT().
					CreateOrUpdateConsumer(gomock.Any(), "test-stream", gomock.Any()).
					Return(nil, errors.New("consumer creation failed")).
					Times(1)
			},
			validateFunc: func(err error) {
				s.EqualError(
					err,
					"error creating consumer for stream test-stream: consumer creation failed",
				)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			tc.validateFunc(s.client.CreateOrUpdateJetStreamWithConfig(
				s.ctx,
				tc.streamConfig,
				tc.consumerConfigs...,
			))
		})
	}
}

func (s *JetStreamPublicTestSuite) TestGetStreamInfo() {
	tests := []struct {
		name         string
		streamName   string
		mockSetup    func()
		validateFunc func(*jetstream.StreamInfo, error)
	}{
		{
			name:       "successfully gets stream info",
			streamName: "TEST-STREAM",
			mockSetup: func() {
				expectedInfo := &jetstream.StreamInfo{
					Config: jetstream.StreamConfig{
						Name:     "TEST-STREAM",
						Subjects: []string{"test.>"},
						Storage:  jetstream.FileStorage,
					},
					State: jetstream.StreamState{
						Msgs:      10,
						Bytes:     1024,
						FirstSeq:  1,
						LastSeq:   10,
						Consumers: 2,
					},
				}
				mockStream := mocks.NewMockStream(s.mockCtrl)
				s.mockExt.EXPECT().
					Stream(gomock.Any(), "TEST-STREAM").
					Return(mockStream, nil).
					Times(1)
				mockStream.EXPECT().
					Info(gomock.Any()).
					Return(expectedInfo, nil).
					Times(1)
			},
			validateFunc: func(info *jetstream.StreamInfo, err error) {
				s.NoError(err)
				s.NotNil(info)
				s.Equal("TEST-STREAM", info.Config.Name)
				s.Equal([]string{"test.>"}, info.Config.Subjects)
				s.Equal(jetstream.FileStorage, info.Config.Storage)
				s.Equal(uint64(10), info.State.Msgs)
				s.Equal(uint64(1024), info.State.Bytes)
				s.Equal(uint64(1), info.State.FirstSeq)
				s.Equal(uint64(10), info.State.LastSeq)
				s.Equal(2, info.State.Consumers)
			},
		},
		{
			name:       "error getting stream - stream not found",
			streamName: "MISSING-STREAM",
			mockSetup: func() {
				s.mockExt.EXPECT().
					Stream(gomock.Any(), "MISSING-STREAM").
					Return(nil, errors.New("stream not found")).
					Times(1)
			},
			validateFunc: func(info *jetstream.StreamInfo, err error) {
				s.Error(err)
				s.Contains(err.Error(), "failed to get stream MISSING-STREAM: stream not found")
				s.Nil(info)
			},
		},
		{
			name:       "error getting stream info - connection error",
			streamName: "ERROR-STREAM",
			mockSetup: func() {
				mockStream := mocks.NewMockStream(s.mockCtrl)
				s.mockExt.EXPECT().
					Stream(gomock.Any(), "ERROR-STREAM").
					Return(mockStream, nil).
					Times(1)
				mockStream.EXPECT().
					Info(gomock.Any()).
					Return(nil, errors.New("connection lost")).
					Times(1)
			},
			validateFunc: func(info *jetstream.StreamInfo, err error) {
				s.Error(err)
				s.Contains(
					err.Error(),
					"failed to get stream info for ERROR-STREAM: connection lost",
				)
				s.Nil(info)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			tc.validateFunc(s.client.GetStreamInfo(s.ctx, tc.streamName))
		})
	}
}

func (s *JetStreamPublicTestSuite) TestPublish() {
	tests := []struct {
		name         string
		subject      string
		data         []byte
		mockSetup    func()
		validateFunc func(error)
	}{
		{
			name:    "successfully publishes message",
			subject: "test.subject",
			data:    []byte("test message"),
			mockSetup: func() {
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:    "publishes empty message",
			subject: "test.empty",
			data:    []byte(""),
			mockSetup: func() {
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:    "error publishing message",
			subject: "test.error",
			data:    []byte("test message"),
			mockSetup: func() {
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("publish failed")).
					Times(1)
			},
			validateFunc: func(err error) {
				s.EqualError(err, "failed to publish message to test.error: publish failed")
			},
		},
		{
			name:    "jetstream not initialized",
			subject: "test.noinit",
			data:    []byte("test message"),
			mockSetup: func() {
				s.client.ExtJS = nil
			},
			validateFunc: func(err error) {
				s.EqualError(err, "jetstream not initialized: call Connect() first")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			tc.validateFunc(s.client.Publish(s.ctx, tc.subject, tc.data))
		})
	}
}

func TestJetStreamPublicTestSuite(t *testing.T) {
	suite.Run(t, new(JetStreamPublicTestSuite))
}

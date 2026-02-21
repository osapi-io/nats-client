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
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, EXPRESS OR IMPLIED,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package client_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type JetStreamPublicTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockJS   *mocks.MockJetStreamContext
	mockExt  *mocks.MockJetStream
	client   *client.Client
	ctx      context.Context
}

func (s *JetStreamPublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockJS = mocks.NewMockJetStreamContext(s.mockCtrl)
	s.mockExt = mocks.NewMockJetStream(s.mockCtrl)
	s.client = client.New(slog.Default(), &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	})
	s.client.NativeJS = s.mockJS
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
		name        string
		config      *nats.StreamConfig
		mockSetup   func()
		expectedErr string
	}{
		{
			name:   "successfully creates stream",
			config: &nats.StreamConfig{Name: "test-stream", Subjects: []string{"test.*"}},
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(gomock.Any()).
					Return(nil, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:   "error creating stream",
			config: &nats.StreamConfig{Name: "test-stream"},
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(gomock.Any()).
					Return(nil, errors.New("stream creation failed")).
					Times(1)
			},
			expectedErr: "error creating stream test-stream: stream creation failed",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			err := s.client.CreateOrUpdateStreamWithConfig(s.ctx, tc.config)

			if tc.expectedErr == "" {
				s.NoError(err)
			} else {
				s.EqualError(err, tc.expectedErr)
			}
		})
	}
}

func (s *JetStreamPublicTestSuite) TestCreateOrUpdateConsumerWithConfig() {
	tests := []struct {
		name        string
		streamName  string
		config      jetstream.ConsumerConfig
		mockSetup   func()
		expectedErr string
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
			expectedErr: "",
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
			expectedErr: "error creating consumer for stream test-stream: consumer creation failed",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			err := s.client.CreateOrUpdateConsumerWithConfig(s.ctx, tc.streamName, tc.config)

			if tc.expectedErr == "" {
				s.NoError(err)
			} else {
				s.EqualError(err, tc.expectedErr)
			}
		})
	}
}

func (s *JetStreamPublicTestSuite) TestCreateOrUpdateJetStreamWithConfig() {
	tests := []struct {
		name            string
		streamConfig    *nats.StreamConfig
		consumerConfigs []jetstream.ConsumerConfig
		mockSetup       func()
		expectedErr     string
	}{
		{
			name:         "successfully creates stream and consumers",
			streamConfig: &nats.StreamConfig{Name: "test-stream", Subjects: []string{"test.*"}},
			consumerConfigs: []jetstream.ConsumerConfig{
				{Durable: "consumer-1"},
				{Durable: "consumer-2"},
			},
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(gomock.Any()).
					Return(nil, nil).
					Times(1)
				s.mockExt.EXPECT().
					CreateOrUpdateConsumer(gomock.Any(), "test-stream", gomock.Any()).
					Return(nil, nil).
					Times(2)
			},
			expectedErr: "",
		},
		{
			name:            "error creating stream",
			streamConfig:    &nats.StreamConfig{Name: "test-stream"},
			consumerConfigs: []jetstream.ConsumerConfig{{Durable: "consumer-1"}},
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(gomock.Any()).
					Return(nil, errors.New("stream creation failed")).
					Times(1)
				s.mockExt.EXPECT().
					CreateOrUpdateConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			expectedErr: "error creating stream test-stream: stream creation failed",
		},
		{
			name:            "error creating consumer",
			streamConfig:    &nats.StreamConfig{Name: "test-stream"},
			consumerConfigs: []jetstream.ConsumerConfig{{Durable: "consumer-1"}},
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(gomock.Any()).
					Return(nil, nil).
					Times(1)
				s.mockExt.EXPECT().
					CreateOrUpdateConsumer(gomock.Any(), "test-stream", gomock.Any()).
					Return(nil, errors.New("consumer creation failed")).
					Times(1)
			},
			expectedErr: "error creating consumer for stream test-stream: consumer creation failed",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			err := s.client.CreateOrUpdateJetStreamWithConfig(
				s.ctx,
				tc.streamConfig,
				tc.consumerConfigs...)

			if tc.expectedErr == "" {
				s.NoError(err)
			} else {
				s.EqualError(err, tc.expectedErr)
			}
		})
	}
}

func (s *JetStreamPublicTestSuite) TestGetStreamInfo() {
	tests := []struct {
		name         string
		streamName   string
		mockSetup    func()
		expectedInfo *nats.StreamInfo
		expectedErr  string
	}{
		{
			name:       "successfully gets stream info",
			streamName: "TEST-STREAM",
			mockSetup: func() {
				expectedInfo := &nats.StreamInfo{
					Config: nats.StreamConfig{
						Name:     "TEST-STREAM",
						Subjects: []string{"test.>"},
						Storage:  nats.FileStorage,
					},
					State: nats.StreamState{
						Msgs:      10,
						Bytes:     1024,
						FirstSeq:  1,
						LastSeq:   10,
						Consumers: 2,
					},
				}
				s.mockJS.EXPECT().
					StreamInfo("TEST-STREAM").
					Return(expectedInfo, nil).
					Times(1)
			},
			expectedInfo: &nats.StreamInfo{
				Config: nats.StreamConfig{
					Name:     "TEST-STREAM",
					Subjects: []string{"test.>"},
					Storage:  nats.FileStorage,
				},
				State: nats.StreamState{
					Msgs:      10,
					Bytes:     1024,
					FirstSeq:  1,
					LastSeq:   10,
					Consumers: 2,
				},
			},
			expectedErr: "",
		},
		{
			name:       "error getting stream info - stream not found",
			streamName: "MISSING-STREAM",
			mockSetup: func() {
				s.mockJS.EXPECT().
					StreamInfo("MISSING-STREAM").
					Return(nil, errors.New("stream not found")).
					Times(1)
			},
			expectedInfo: nil,
			expectedErr:  "failed to get stream info for MISSING-STREAM: stream not found",
		},
		{
			name:       "error getting stream info - connection error",
			streamName: "ERROR-STREAM",
			mockSetup: func() {
				s.mockJS.EXPECT().
					StreamInfo("ERROR-STREAM").
					Return(nil, errors.New("connection lost")).
					Times(1)
			},
			expectedInfo: nil,
			expectedErr:  "failed to get stream info for ERROR-STREAM: connection lost",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			info, err := s.client.GetStreamInfo(s.ctx, tc.streamName)

			if tc.expectedErr == "" {
				s.NoError(err)
				s.NotNil(info)
				s.Equal(tc.expectedInfo.Config.Name, info.Config.Name)
				s.Equal(tc.expectedInfo.Config.Subjects, info.Config.Subjects)
				s.Equal(tc.expectedInfo.Config.Storage, info.Config.Storage)
				s.Equal(tc.expectedInfo.State.Msgs, info.State.Msgs)
				s.Equal(tc.expectedInfo.State.Bytes, info.State.Bytes)
				s.Equal(tc.expectedInfo.State.FirstSeq, info.State.FirstSeq)
				s.Equal(tc.expectedInfo.State.LastSeq, info.State.LastSeq)
				s.Equal(tc.expectedInfo.State.Consumers, info.State.Consumers)
			} else {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedErr)
				s.Nil(info)
			}
		})
	}
}

func (s *JetStreamPublicTestSuite) TestPublish() {
	tests := []struct {
		name        string
		subject     string
		data        []byte
		mockSetup   func()
		expectedErr string
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
			expectedErr: "",
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
			expectedErr: "",
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
			expectedErr: "failed to publish message to test.error: publish failed",
		},
		{
			name:        "jetstream not initialized",
			subject:     "test.noinit",
			data:        []byte("test message"),
			mockSetup:   func() {},
			expectedErr: "JetStream not initialized: call Connect() first",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			// For the "jetstream not initialized" test, set ExtJS to nil
			if tc.name == "jetstream not initialized" {
				originalExtJS := s.client.ExtJS
				s.client.ExtJS = nil
				defer func() {
					s.client.ExtJS = originalExtJS
				}()
			}

			err := s.client.Publish(s.ctx, tc.subject, tc.data)

			if tc.expectedErr == "" {
				s.NoError(err)
			} else {
				s.EqualError(err, tc.expectedErr)
			}
		})
	}
}

func TestJetStreamPublicTestSuite(t *testing.T) {
	suite.Run(t, new(JetStreamPublicTestSuite))
}

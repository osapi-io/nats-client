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

func (s *JetStreamPublicTestSuite) TestCreateOrUpdateJetStream() {
	tests := []struct {
		name        string
		streams     []*client.StreamConfig
		mockSetup   func()
		expectedErr string
	}{
		{
			name:        "returns error if no stream configs are provided",
			streams:     []*client.StreamConfig{},
			mockSetup:   func() {},
			expectedErr: "jetstream is enabled but no stream configuration was provided",
		},
		{
			name: "successfully configures streams and consumers",
			streams: []*client.StreamConfig{
				{
					StreamConfig: &nats.StreamConfig{Name: "test-stream"},
					Consumers: []*client.ConsumerConfig{
						{ConsumerConfig: &jetstream.ConsumerConfig{Durable: "consumer-1"}},
						{ConsumerConfig: &jetstream.ConsumerConfig{Durable: "consumer-2"}},
					},
				},
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
			name: "error creating or updating stream",
			streams: []*client.StreamConfig{
				{
					StreamConfig: &nats.StreamConfig{Name: "test-stream"},
					Consumers: []*client.ConsumerConfig{
						{
							ConsumerConfig: &jetstream.ConsumerConfig{
								Durable: "consumer-1",
							},
						},
					},
				},
			},
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
			name: "error creating consumer",
			streams: []*client.StreamConfig{
				{
					StreamConfig: &nats.StreamConfig{Name: "test-stream"},
					Consumers: []*client.ConsumerConfig{
						{
							ConsumerConfig: &jetstream.ConsumerConfig{
								Durable: "consumer-1",
							},
						},
					},
				},
			},
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

			err := s.client.CreateOrUpdateJetStream(s.ctx, tc.streams...)

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

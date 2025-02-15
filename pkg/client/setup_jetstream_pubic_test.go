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
	"errors"
	"log/slog"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type JetStreamPublicTestSuite struct {
	suite.Suite
	ctrl *gomock.Controller

	js *mocks.MockJetStreamContext
}

func (suite *JetStreamPublicTestSuite) SetupTest() {
	suite.ctrl = gomock.NewController(suite.T())

	suite.js = mocks.NewMockJetStreamContext(suite.ctrl)
}

func (suite *JetStreamPublicTestSuite) SetupSubTest() {
	suite.SetupTest()
}

func (suite *JetStreamPublicTestSuite) TearDownTest() {
	suite.ctrl.Finish()
}

func (suite *JetStreamPublicTestSuite) TestSetupJetStreamTable() {
	tests := []struct {
		name       string
		streams    []*client.StreamConfig
		setupMocks func()
		expectErr  string
	}{
		{
			name: "success",
			streams: []*client.StreamConfig{
				{
					StreamConfig: &nats.StreamConfig{
						Name:     "test-stream",
						Subjects: []string{"test.subject"},
					},
					Consumers: []*client.ConsumerConfig{
						{ConsumerConfig: &nats.ConsumerConfig{Durable: "test-consumer"}},
					},
				},
			},
			setupMocks: func() {
				suite.js.EXPECT().AddStream(gomock.Any()).Return(&nats.StreamInfo{}, nil).Times(1)
				suite.js.EXPECT().
					AddConsumer("test-stream", gomock.Any()).
					Return(&nats.ConsumerInfo{}, nil).
					Times(1)
			},
			expectErr: "",
		},
		{
			name:       "no stream configuration",
			streams:    nil,
			setupMocks: func() {},
			expectErr:  "jetstream is enabled but no stream configuration was provided",
		},
		{
			name: "fail to add stream",
			streams: []*client.StreamConfig{
				{
					StreamConfig: &nats.StreamConfig{
						Name:     "test-stream",
						Subjects: []string{"test.subject"},
					},
					Consumers: []*client.ConsumerConfig{
						{ConsumerConfig: &nats.ConsumerConfig{Durable: "test-consumer"}},
					},
				},
			},
			setupMocks: func() {
				suite.js.EXPECT().
					AddStream(gomock.Any()).
					Return(nil, errors.New("failed to add stream"))
			},
			expectErr: "failed to add stream",
		},
		{
			name: "fail to add consumer",
			streams: []*client.StreamConfig{
				{
					StreamConfig: &nats.StreamConfig{
						Name:     "test-stream",
						Subjects: []string{"test.subject"},
					},
					Consumers: []*client.ConsumerConfig{
						{ConsumerConfig: &nats.ConsumerConfig{Durable: "test-consumer"}},
					},
				},
			},
			setupMocks: func() {
				suite.js.EXPECT().AddStream(gomock.Any()).Return(&nats.StreamInfo{}, nil).Times(1)
				suite.js.EXPECT().
					AddConsumer("test-stream", gomock.Any()).
					Return(nil, errors.New("failed to add consumer"))
			},
			expectErr: "failed to add consumer",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			c := client.New(slog.Default(), tc.streams...)
			tc.setupMocks()
			err := c.SetupJetStream(suite.js)
			if tc.expectErr == "" {
				suite.NoError(err)
			} else {
				suite.ErrorContains(err, tc.expectErr)
			}
		})
	}
}

func TestSetupJetStreamPublicTestSuite(t *testing.T) {
	suite.Run(t, new(JetStreamPublicTestSuite))
}

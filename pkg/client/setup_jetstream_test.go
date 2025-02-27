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

package client

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type JetStreamTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockJS   *mocks.MockJetStreamContext
	client   *Client
}

func (s *JetStreamTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockJS = mocks.NewMockJetStreamContext(s.mockCtrl)
	s.client = New(slog.Default(), &Options{
		Host: "localhost",
		Port: 4222,
		Auth: AuthOptions{
			AuthType: NoAuth,
		},
	})
	s.client.NativeJS = s.mockJS
}

func (suite *JetStreamTestSuite) SetupSubTest() {
	suite.SetupTest()
}

func (s *JetStreamTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *JetStreamTestSuite) TestCreateOrUpdateStream() {
	tests := []struct {
		name         string
		streamConfig *nats.StreamConfig
		streamName   string
		mockSetup    func()
		expectedErr  string
	}{
		{
			name:         "successfully creates stream",
			streamConfig: &nats.StreamConfig{Name: "test-stream"},
			streamName:   "test-stream",
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(&nats.StreamConfig{Name: "test-stream"}).
					Return(&nats.StreamInfo{}, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:         "fails to create stream",
			streamConfig: &nats.StreamConfig{Name: "test-stream"},
			streamName:   "test-stream",
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(&nats.StreamConfig{Name: "test-stream"}).
					Return(nil, errors.New("nats: error creating stream")).
					Times(1)
			},
			expectedErr: "error creating stream test-stream: nats: error creating stream",
		},
		{
			name:         "stream already exists, successfully updates",
			streamConfig: &nats.StreamConfig{Name: "test-stream"},
			streamName:   "test-stream",
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(&nats.StreamConfig{Name: "test-stream"}).
					Return(nil, errors.New("stream name already in use")).
					Times(1)
				s.mockJS.EXPECT().
					UpdateStream(&nats.StreamConfig{Name: "test-stream"}).
					Return(&nats.StreamInfo{}, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:         "stream already exists, fails to update",
			streamConfig: &nats.StreamConfig{Name: "test-stream"},
			streamName:   "test-stream",
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(&nats.StreamConfig{Name: "test-stream"}).
					Return(nil, errors.New("stream name already in use")).
					Times(1)

				s.mockJS.EXPECT().
					UpdateStream(&nats.StreamConfig{Name: "test-stream"}).
					Return(nil, errors.New("nats: error updating stream")).
					Times(1)
			},
			expectedErr: "error updating stream test-stream: nats: error updating stream",
		},
		{
			name:         "fails to create stream",
			streamConfig: &nats.StreamConfig{Name: "test-stream"},
			streamName:   "test-stream",
			mockSetup: func() {
				s.mockJS.EXPECT().
					AddStream(gomock.Any()).
					Return(nil, errors.New("nats: error creating stream")).
					Times(1)
			},
			expectedErr: "error creating stream test-stream: nats: error creating stream",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.mockCtrl = gomock.NewController(s.T())
			s.mockJS = mocks.NewMockJetStreamContext(s.mockCtrl)
			s.client.NativeJS = s.mockJS

			tc.mockSetup()

			err := s.client.createOrUpdateStream(tc.streamConfig, tc.streamName)

			if tc.expectedErr == "" {
				s.NoError(err)
			} else {
				s.EqualError(err, tc.expectedErr)
			}
		})
	}
}

func TestSetupJetStreamTestSuite(t *testing.T) {
	suite.Run(t, new(JetStreamTestSuite))
}

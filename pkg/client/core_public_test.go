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
	"errors"
	"log/slog"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type CorePublicTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockNATS *mocks.MockNATSConnector
	client   *client.Client
}

func (s *CorePublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockNATS = mocks.NewMockNATSConnector(s.mockCtrl)
	s.client = client.New(slog.Default(), &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	})
	s.client.NC = s.mockNATS
}

func (s *CorePublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *CorePublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *CorePublicTestSuite) TestSubscribe() {
	tests := []struct {
		name        string
		subject     string
		setupMock   func()
		wantErr     bool
		errContains string
	}{
		{
			name:    "when subscribe succeeds returns subscription",
			subject: "test.subject",
			setupMock: func() {
				s.mockNATS.EXPECT().
					Subscribe("test.subject", gomock.Any()).
					Return(&nats.Subscription{}, nil)
			},
		},
		{
			name:    "when subscribe fails returns error",
			subject: "test.subject",
			setupMock: func() {
				s.mockNATS.EXPECT().
					Subscribe("test.subject", gomock.Any()).
					Return(nil, errors.New("connection closed"))
			},
			wantErr:     true,
			errContains: "failed to subscribe",
		},
		{
			name:    "when NC is nil returns error",
			subject: "test.subject",
			setupMock: func() {
				s.client.NC = nil
			},
			wantErr:     true,
			errContains: "NATS connection not established",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			sub, err := s.client.Subscribe(tt.subject, func(_ *nats.Msg) {})

			if tt.wantErr {
				s.Error(err)
				s.Contains(err.Error(), tt.errContains)
				s.Nil(sub)
			} else {
				s.NoError(err)
				s.NotNil(sub)
			}
		})
	}
}

func (s *CorePublicTestSuite) TestPublishCore() {
	tests := []struct {
		name        string
		subject     string
		data        []byte
		setupClient func()
		wantErr     bool
		errContains string
	}{
		{
			name:    "when publish succeeds",
			subject: "test.subject",
			data:    []byte("test data"),
			setupClient: func() {
				s.mockNATS.EXPECT().
					Publish("test.subject", []byte("test data")).
					Return(nil)
			},
		},
		{
			name:    "when publish fails returns error",
			subject: "test.subject",
			data:    []byte("test data"),
			setupClient: func() {
				s.mockNATS.EXPECT().
					Publish("test.subject", []byte("test data")).
					Return(errors.New("connection closed"))
			},
			wantErr:     true,
			errContains: "failed to publish",
		},
		{
			name:    "when NC is nil returns error",
			subject: "test.subject",
			data:    []byte("test data"),
			setupClient: func() {
				s.client.NC = nil
			},
			wantErr:     true,
			errContains: "NATS connection not established",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.setupClient != nil {
				tt.setupClient()
			}

			err := s.client.PublishCore(tt.subject, tt.data)

			if tt.wantErr {
				s.Error(err)
				s.Contains(err.Error(), tt.errContains)
			} else {
				s.NoError(err)
			}
		})
	}
}

func TestCorePublicTestSuite(t *testing.T) {
	suite.Run(t, new(CorePublicTestSuite))
}

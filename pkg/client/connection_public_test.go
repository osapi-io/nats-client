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
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type ConnectionPublicTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockNATS *mocks.MockNATSConnector
	mockJS   *mocks.MockJetStream
	client   *client.Client
}

func (s *ConnectionPublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockNATS = mocks.NewMockNATSConnector(s.mockCtrl)
	s.mockJS = mocks.NewMockJetStream(s.mockCtrl)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	s.client = client.New(logger, &client.Options{
		Host: "localhost",
		Port: 4222,
	})
	s.client.NC = s.mockNATS
	s.client.ExtJS = s.mockJS
}

func (s *ConnectionPublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *ConnectionPublicTestSuite) TestClose() {
	tests := []struct {
		name      string
		setupFunc func()
	}{
		{
			name: "closes the underlying connection",
			setupFunc: func() {
				s.mockNATS.EXPECT().Close()
			},
		},
		{
			name: "handles nil NC gracefully",
			setupFunc: func() {
				s.client.NC = nil
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.setupFunc()
			s.client.Close()
		})
	}
}

func (s *ConnectionPublicTestSuite) TestConnectedURL() {
	tests := []struct {
		name         string
		setupFunc    func()
		validateFunc func(string)
	}{
		{
			name: "returns connected URL",
			setupFunc: func() {
				s.mockNATS.EXPECT().
					ConnectedUrl().
					Return("nats://localhost:4222")
			},
			validateFunc: func(url string) {
				s.Equal("nats://localhost:4222", url)
			},
		},
		{
			name: "returns empty string when NC is nil",
			setupFunc: func() {
				s.client.NC = nil
			},
			validateFunc: func(url string) {
				s.Empty(url)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.setupFunc()
			tc.validateFunc(s.client.ConnectedURL())
		})
	}
}

func (s *ConnectionPublicTestSuite) TestConnectedServerVersion() {
	tests := []struct {
		name         string
		setupFunc    func()
		validateFunc func(string)
	}{
		{
			name: "returns empty string when NC is mock (not NATSConnWrapper)",
			setupFunc: func() {
				// mockNATS is a MockNATSConnector, not a *NATSConnWrapper
			},
			validateFunc: func(version string) {
				s.Empty(version)
			},
		},
		{
			name: "returns empty string when NC is nil",
			setupFunc: func() {
				s.client.NC = nil
			},
			validateFunc: func(version string) {
				s.Empty(version)
			},
		},
		{
			name: "returns empty string when NATSConnWrapper has nil Conn",
			setupFunc: func() {
				s.client.NC = &client.NATSConnWrapper{Conn: nil}
			},
			validateFunc: func(version string) {
				s.Empty(version)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.setupFunc()
			tc.validateFunc(s.client.ConnectedServerVersion())
		})
	}
}

func (s *ConnectionPublicTestSuite) TestKeyValue() {
	tests := []struct {
		name         string
		bucket       string
		setupFunc    func()
		validateFunc func(jetstream.KeyValue, error)
	}{
		{
			name:   "returns KV bucket handle",
			bucket: "test-bucket",
			setupFunc: func() {
				mockKV := mocks.NewMockKeyValue(s.mockCtrl)
				s.mockJS.EXPECT().
					KeyValue(gomock.Any(), "test-bucket").
					Return(mockKV, nil)
			},
			validateFunc: func(kv jetstream.KeyValue, err error) {
				s.NoError(err)
				s.NotNil(kv)
			},
		},
		{
			name:   "returns error when bucket not found",
			bucket: "missing",
			setupFunc: func() {
				s.mockJS.EXPECT().
					KeyValue(gomock.Any(), "missing").
					Return(nil, errors.New("bucket not found"))
			},
			validateFunc: func(kv jetstream.KeyValue, err error) {
				s.Error(err)
				s.Nil(kv)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.setupFunc()
			kv, err := s.client.KeyValue(context.Background(), tc.bucket)
			tc.validateFunc(kv, err)
		})
	}
}

func (s *ConnectionPublicTestSuite) TestStream() {
	tests := []struct {
		name         string
		streamName   string
		setupFunc    func()
		validateFunc func(jetstream.Stream, error)
	}{
		{
			name:       "returns stream handle",
			streamName: "JOBS",
			setupFunc: func() {
				mockStream := mocks.NewMockStream(s.mockCtrl)
				s.mockJS.EXPECT().
					Stream(gomock.Any(), "JOBS").
					Return(mockStream, nil)
			},
			validateFunc: func(stream jetstream.Stream, err error) {
				s.NoError(err)
				s.NotNil(stream)
			},
		},
		{
			name:       "returns error when stream not found",
			streamName: "MISSING",
			setupFunc: func() {
				s.mockJS.EXPECT().
					Stream(gomock.Any(), "MISSING").
					Return(nil, errors.New("stream not found"))
			},
			validateFunc: func(stream jetstream.Stream, err error) {
				s.Error(err)
				s.Nil(stream)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.setupFunc()
			stream, err := s.client.Stream(context.Background(), tc.streamName)
			tc.validateFunc(stream, err)
		})
	}
}

func TestConnectionPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionPublicTestSuite))
}

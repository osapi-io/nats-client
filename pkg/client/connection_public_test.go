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
	"log/slog"
	"os"
	"testing"

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

func TestConnectionPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionPublicTestSuite))
}

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
	"fmt"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type ConsumerTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	client   *Client
	logger   *slog.Logger
}

func (s *ConsumerTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.logger = slog.Default()

	s.client = &Client{
		logger: s.logger,
		Opts: &Options{
			Host: "localhost",
			Port: 4222,
			Auth: AuthOptions{
				AuthType: NoAuth,
			},
		},
	}
}

func (s *ConsumerTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *ConsumerTestSuite) TestProcessMessage() {
	tests := []struct {
		name        string
		handler     JetStreamMessageHandler
		setupMsg    func() jetstream.Msg
		expectedErr string
	}{
		{
			name: "successful handler execution",
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			setupMsg: func() jetstream.Msg {
				mockMsg := mocks.NewMockMsg(s.mockCtrl)
				mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
				return mockMsg
			},
			expectedErr: "",
		},
		{
			name: "handler returns error",
			handler: func(_ jetstream.Msg) error {
				return errors.New("processing failed")
			},
			setupMsg: func() jetstream.Msg {
				mockMsg := mocks.NewMockMsg(s.mockCtrl)
				mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
				return mockMsg
			},
			expectedErr: "processing failed",
		},
		{
			name: "handler panics with string",
			handler: func(_ jetstream.Msg) error {
				panic("unexpected failure")
			},
			setupMsg: func() jetstream.Msg {
				mockMsg := mocks.NewMockMsg(s.mockCtrl)
				mockMsg.EXPECT().Subject().Return("panic.subject").AnyTimes()
				return mockMsg
			},
			expectedErr: "handler panicked: unexpected failure",
		},
		{
			name: "handler panics with error",
			handler: func(_ jetstream.Msg) error {
				panic(errors.New("panic error"))
			},
			setupMsg: func() jetstream.Msg {
				mockMsg := mocks.NewMockMsg(s.mockCtrl)
				mockMsg.EXPECT().Subject().Return("panic.error.subject").AnyTimes()
				return mockMsg
			},
			expectedErr: "handler panicked: panic error",
		},
		{
			name: "handler panics with integer",
			handler: func(_ jetstream.Msg) error {
				panic(42)
			},
			setupMsg: func() jetstream.Msg {
				mockMsg := mocks.NewMockMsg(s.mockCtrl)
				mockMsg.EXPECT().Subject().Return("panic.int.subject").AnyTimes()
				return mockMsg
			},
			expectedErr: "handler panicked: 42",
		},
		{
			name: "handler returns wrapped error",
			handler: func(_ jetstream.Msg) error {
				return fmt.Errorf("outer: %w", errors.New("inner error"))
			},
			setupMsg: func() jetstream.Msg {
				mockMsg := mocks.NewMockMsg(s.mockCtrl)
				mockMsg.EXPECT().Subject().Return("wrapped.subject").AnyTimes()
				return mockMsg
			},
			expectedErr: "outer: inner error",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			msg := tc.setupMsg()
			err := s.client.processMessage(msg, tc.handler)

			if tc.expectedErr != "" {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedErr)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func TestConsumerTestSuite(t *testing.T) {
	suite.Run(t, new(ConsumerTestSuite))
}

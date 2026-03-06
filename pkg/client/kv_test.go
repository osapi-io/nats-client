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
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type KVTestSuite struct {
	suite.Suite

	client *Client
	logger *slog.Logger
}

func (s *KVTestSuite) SetupTest() {
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

func (s *KVTestSuite) TestWaitForKVResponse() {
	tests := []struct {
		name         string
		setupMocks   func(ctrl *gomock.Controller) *mocks.MockKeyValue
		pollInterval time.Duration
		ctxTimeout   time.Duration
		expectedData []byte
		expectedErr  string
	}{
		{
			name: "key found on first poll",
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockKeyValue {
				mockKV := mocks.NewMockKeyValue(ctrl)
				mockEntry := mocks.NewMockKeyValueEntry(ctrl)
				mockEntry.EXPECT().Value().Return([]byte("response-data"))
				mockEntry.EXPECT().Revision().Return(uint64(1))

				mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(mockEntry, nil).
					Times(1)

				return mockKV
			},
			pollInterval: 10 * time.Millisecond,
			ctxTimeout:   500 * time.Millisecond,
			expectedData: []byte("response-data"),
			expectedErr:  "",
		},
		{
			name: "key not found then found on subsequent poll",
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockKeyValue {
				mockKV := mocks.NewMockKeyValue(ctrl)
				mockEntry := mocks.NewMockKeyValueEntry(ctrl)
				mockEntry.EXPECT().Value().Return([]byte("delayed-response"))
				mockEntry.EXPECT().Revision().Return(uint64(2))

				first := mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(nil, jetstream.ErrKeyNotFound).
					Times(1)

				mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(mockEntry, nil).
					Times(1).
					After(first)

				return mockKV
			},
			pollInterval: 10 * time.Millisecond,
			ctxTimeout:   500 * time.Millisecond,
			expectedData: []byte("delayed-response"),
			expectedErr:  "",
		},
		{
			name: "context timeout before key found",
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockKeyValue {
				mockKV := mocks.NewMockKeyValue(ctrl)

				mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(nil, jetstream.ErrKeyNotFound).
					AnyTimes()

				return mockKV
			},
			pollInterval: 10 * time.Millisecond,
			ctxTimeout:   50 * time.Millisecond,
			expectedData: nil,
			expectedErr:  "timeout waiting for response",
		},
		{
			name: "non-retryable KV get error",
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockKeyValue {
				mockKV := mocks.NewMockKeyValue(ctrl)

				mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(nil, errors.New("connection lost")).
					Times(1)

				return mockKV
			},
			pollInterval: 10 * time.Millisecond,
			ctxTimeout:   500 * time.Millisecond,
			expectedData: nil,
			expectedErr:  "failed to get response",
		},
		{
			name: "empty value returned",
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockKeyValue {
				mockKV := mocks.NewMockKeyValue(ctrl)
				mockEntry := mocks.NewMockKeyValueEntry(ctrl)
				mockEntry.EXPECT().Value().Return([]byte{})
				mockEntry.EXPECT().Revision().Return(uint64(1))

				mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(mockEntry, nil).
					Times(1)

				return mockKV
			},
			pollInterval: 10 * time.Millisecond,
			ctxTimeout:   500 * time.Millisecond,
			expectedData: []byte{},
			expectedErr:  "",
		},
		{
			name: "multiple not-found before success",
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockKeyValue {
				mockKV := mocks.NewMockKeyValue(ctrl)
				mockEntry := mocks.NewMockKeyValueEntry(ctrl)
				mockEntry.EXPECT().Value().Return([]byte("eventually-found"))
				mockEntry.EXPECT().Revision().Return(uint64(5))

				first := mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(nil, jetstream.ErrKeyNotFound).
					Times(1)

				second := mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(nil, jetstream.ErrKeyNotFound).
					Times(1).
					After(first)

				third := mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(nil, jetstream.ErrKeyNotFound).
					Times(1).
					After(second)

				mockKV.EXPECT().
					Get(gomock.Any(), "request-123").
					Return(mockEntry, nil).
					Times(1).
					After(third)

				return mockKV
			},
			pollInterval: 10 * time.Millisecond,
			ctxTimeout:   500 * time.Millisecond,
			expectedData: []byte("eventually-found"),
			expectedErr:  "",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			ctrl := gomock.NewController(s.T())
			defer ctrl.Finish()

			mockKV := tc.setupMocks(ctrl)

			ctx, cancel := context.WithTimeout(context.Background(), tc.ctxTimeout)
			defer cancel()

			data, err := s.client.waitForKVResponse(
				ctx,
				mockKV,
				"request-123",
				tc.pollInterval,
			)

			if tc.expectedErr != "" {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedErr)
				s.Nil(data)
			} else {
				s.Require().NoError(err)
				s.Equal(tc.expectedData, data)
			}
		})
	}
}

func TestKVTestSuite(t *testing.T) {
	suite.Run(t, new(KVTestSuite))
}

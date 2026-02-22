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
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type KVStreamPublicTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockExt  *mocks.MockJetStream
	mockKV   *mocks.MockKeyValue
	client   *client.Client
}

func (s *KVStreamPublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockExt = mocks.NewMockJetStream(s.mockCtrl)
	s.mockKV = mocks.NewMockKeyValue(s.mockCtrl)
	s.client = client.New(slog.Default(), &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	})
	s.client.ExtJS = s.mockExt
}

func (s *KVStreamPublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *KVStreamPublicTestSuite) TestKVPutAndPublish() {
	tests := []struct {
		name           string
		kvBucket       string
		key            string
		data           []byte
		notifySubject  string
		mockSetup      func()
		expectedResult uint64
		expectedError  string
	}{
		{
			name:          "successfully stores and notifies",
			kvBucket:      "test-bucket",
			key:           "test-key",
			data:          []byte(`{"test": "data"}`),
			notifySubject: "notify.subject",
			mockSetup: func() {
				// Mock KV bucket retrieval
				s.mockExt.EXPECT().
					CreateOrUpdateKeyValue(gomock.Any(), jetstream.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil)

				// Mock KV put
				s.mockKV.EXPECT().
					Put(gomock.Any(), "test-key", []byte(`{"test": "data"}`)).
					Return(uint64(42), nil)

				// Mock stream publish
				s.mockExt.EXPECT().
					Publish(gomock.Any(), "notify.subject", []byte("test-key")).
					Return(nil, nil)
			},
			expectedResult: 42,
		},
		{
			name:          "error getting KV bucket",
			kvBucket:      "bad-bucket",
			key:           "test-key",
			data:          []byte(`{"test": "data"}`),
			notifySubject: "notify.subject",
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateKeyValue(gomock.Any(), jetstream.KeyValueConfig{Bucket: "bad-bucket"}).
					Return(nil, errors.New("bucket not found"))
			},
			expectedError: "failed to get KV bucket 'bad-bucket': failed to create/update KV bucket bad-bucket: bucket not found",
		},
		{
			name:          "error storing in KV",
			kvBucket:      "test-bucket",
			key:           "test-key",
			data:          []byte(`{"test": "data"}`),
			notifySubject: "notify.subject",
			mockSetup: func() {
				// KV bucket retrieval succeeds
				s.mockExt.EXPECT().
					CreateOrUpdateKeyValue(gomock.Any(), jetstream.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil)

				// KV put fails
				s.mockKV.EXPECT().
					Put(gomock.Any(), "test-key", []byte(`{"test": "data"}`)).
					Return(uint64(0), errors.New("put failed"))
			},
			expectedError: "failed to store data in KV: put failed",
		},
		{
			name:          "error sending notification",
			kvBucket:      "test-bucket",
			key:           "test-key",
			data:          []byte(`{"test": "data"}`),
			notifySubject: "notify.subject",
			mockSetup: func() {
				// KV operations succeed
				s.mockExt.EXPECT().
					CreateOrUpdateKeyValue(gomock.Any(), jetstream.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil)

				s.mockKV.EXPECT().
					Put(gomock.Any(), "test-key", []byte(`{"test": "data"}`)).
					Return(uint64(42), nil)

				// Publish fails
				s.mockExt.EXPECT().
					Publish(gomock.Any(), "notify.subject", []byte("test-key")).
					Return(nil, errors.New("publish failed"))
			},
			expectedError: "failed to send notification: publish failed",
		},
		{
			name:          "handles empty data",
			kvBucket:      "test-bucket",
			key:           "empty-key",
			data:          []byte{},
			notifySubject: "notify.empty",
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateKeyValue(gomock.Any(), jetstream.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil)

				s.mockKV.EXPECT().
					Put(gomock.Any(), "empty-key", []byte{}).
					Return(uint64(1), nil)

				s.mockExt.EXPECT().
					Publish(gomock.Any(), "notify.empty", []byte("empty-key")).
					Return(nil, nil)
			},
			expectedResult: 1,
		},
		{
			name:          "handles nil data",
			kvBucket:      "test-bucket",
			key:           "nil-key",
			data:          nil,
			notifySubject: "notify.nil",
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateKeyValue(gomock.Any(), jetstream.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil)

				s.mockKV.EXPECT().
					Put(gomock.Any(), "nil-key", nil).
					Return(uint64(2), nil)

				s.mockExt.EXPECT().
					Publish(gomock.Any(), "notify.nil", []byte("nil-key")).
					Return(nil, nil)
			},
			expectedResult: 2,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			revision, err := s.client.KVPutAndPublish(
				context.TODO(),
				tc.kvBucket,
				tc.key,
				tc.data,
				tc.notifySubject,
			)

			if tc.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedError)
				s.Equal(uint64(0), revision)
			} else {
				s.NoError(err)
				s.Equal(tc.expectedResult, revision)
			}
		})
	}
}

func TestKVStreamPublicTestSuite(t *testing.T) {
	suite.Run(t, new(KVStreamPublicTestSuite))
}

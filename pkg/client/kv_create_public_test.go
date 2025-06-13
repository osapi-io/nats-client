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
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type KVCreatePublicTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockJS   *mocks.MockJetStreamContext
	mockExt  *mocks.MockJetStream
	mockKV   *mocks.MockKeyValue
	client   *client.Client
}

func (s *KVCreatePublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockJS = mocks.NewMockJetStreamContext(s.mockCtrl)
	s.mockExt = mocks.NewMockJetStream(s.mockCtrl)
	s.mockKV = mocks.NewMockKeyValue(s.mockCtrl)
	s.client = client.New(slog.Default(), &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	})
	s.client.NativeJS = s.mockJS
	s.client.ExtJS = s.mockExt
}

func (s *KVCreatePublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *KVCreatePublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *KVCreatePublicTestSuite) TestCreateKVBucket() {
	tests := []struct {
		name        string
		bucketName  string
		mockSetup   func()
		expectedErr string
	}{
		{
			name:       "successfully creates KV bucket",
			bucketName: "responses",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "responses"}).
					Return(s.mockKV, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:       "error creating KV bucket",
			bucketName: "responses",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "responses"}).
					Return(nil, errors.New("kv creation failed")).
					Times(1)
			},
			expectedErr: "kv creation failed",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			kv, err := s.client.CreateKVBucket(tc.bucketName)

			if tc.expectedErr == "" {
				s.NoError(err)
				s.NotNil(kv)
			} else {
				s.EqualError(err, tc.expectedErr)
			}
		})
	}
}

func (s *KVCreatePublicTestSuite) TestCreateKVBucketWithConfig() {
	tests := []struct {
		name        string
		config      *nats.KeyValueConfig
		mockSetup   func()
		expectedErr string
	}{
		{
			name: "successfully creates KV bucket with custom config",
			config: &nats.KeyValueConfig{
				Bucket:      "job-responses",
				Description: "Storage for job responses",
				TTL:         1 * time.Hour,
				MaxBytes:    100 * 1024 * 1024,
				Storage:     nats.FileStorage,
				Replicas:    1,
			},
			mockSetup: func() {
				expectedConfig := &nats.KeyValueConfig{
					Bucket:      "job-responses",
					Description: "Storage for job responses",
					TTL:         1 * time.Hour,
					MaxBytes:    100 * 1024 * 1024,
					Storage:     nats.FileStorage,
					Replicas:    1,
				}
				s.mockJS.EXPECT().
					CreateKeyValue(expectedConfig).
					Return(s.mockKV, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name: "error creating KV bucket with config",
			config: &nats.KeyValueConfig{
				Bucket: "invalid-bucket",
			},
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "invalid-bucket"}).
					Return(nil, errors.New("invalid bucket configuration")).
					Times(1)
			},
			expectedErr: "invalid bucket configuration",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			kv, err := s.client.CreateKVBucketWithConfig(tc.config)

			if tc.expectedErr == "" {
				s.NoError(err)
				s.NotNil(kv)
			} else {
				s.EqualError(err, tc.expectedErr)
			}
		})
	}
}

func TestKVCreatePublicTestSuite(t *testing.T) {
	suite.Run(t, new(KVCreatePublicTestSuite))
}

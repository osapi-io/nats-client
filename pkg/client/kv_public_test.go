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

func (s *KVCreatePublicTestSuite) TestPublishAndWaitKV() {
	tests := []struct {
		name         string
		subject      string
		data         []byte
		requestID    string
		responseData []byte
		mockSetup    func()
		expectedErr  string
	}{
		{
			name:         "successfully publishes and receives response",
			subject:      "test.subject",
			data:         []byte(`{"test": "data"}`),
			requestID:    "test-123",
			responseData: []byte(`{"status": "ok"}`),
			mockSetup: func() {
				// Mock publish
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)

				// Mock KV get (first call returns not found, second returns data)
				s.mockKV.EXPECT().
					Get("test-123").
					Return(nil, nats.ErrKeyNotFound).
					Times(1)

				mockEntry := mocks.NewMockKeyValueEntry(s.mockCtrl)
				mockEntry.EXPECT().Value().Return([]byte(`{"status": "ok"}`))
				mockEntry.EXPECT().Revision().Return(uint64(1))

				s.mockKV.EXPECT().
					Get("test-123").
					Return(mockEntry, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:      "error publishing message",
			subject:   "test.subject",
			data:      []byte(`{"test": "data"}`),
			requestID: "test-123",
			mockSetup: func() {
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("publish failed")).
					Times(1)
			},
			expectedErr: "failed to publish: publish failed",
		},
		{
			name:         "timeout waiting for response",
			subject:      "test.timeout",
			data:         []byte(`{"test": "timeout"}`),
			requestID:    "timeout-123",
			responseData: nil,
			mockSetup: func() {
				// Mock successful publish
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)

				// Mock KV get returns not found repeatedly (simulating timeout)
				s.mockKV.EXPECT().
					Get("timeout-123").
					Return(nil, nats.ErrKeyNotFound).
					MinTimes(2) // At least 2 calls due to polling
			},
			expectedErr: "context deadline exceeded",
		},
		{
			name:         "context cancelled during wait",
			subject:      "test.cancel",
			data:         []byte(`{"test": "cancel"}`),
			requestID:    "cancel-123",
			responseData: nil,
			mockSetup: func() {
				// Mock successful publish
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)

				// Mock KV get returns not found (will be cancelled)
				s.mockKV.EXPECT().
					Get("cancel-123").
					Return(nil, nats.ErrKeyNotFound).
					AnyTimes()
			},
			expectedErr: "context canceled",
		},
		{
			name:         "error during KV get",
			subject:      "test.kv_error",
			data:         []byte(`{"test": "kv_error"}`),
			requestID:    "kv-error-123",
			responseData: nil,
			mockSetup: func() {
				// Mock successful publish
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)

				// Mock KV get returns an actual error (not ErrKeyNotFound)
				s.mockKV.EXPECT().
					Get("kv-error-123").
					Return(nil, errors.New("kv store error")).
					Times(1)
			},
			expectedErr: "failed to get response: kv store error",
		},
		{
			name:         "nil options uses defaults",
			subject:      "test.defaults",
			data:         []byte(`{"test": "defaults"}`),
			requestID:    "", // Will be generated
			responseData: []byte(`{"status": "ok"}`),
			mockSetup: func() {
				// Mock publish
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)

				// Mock KV get returns response immediately
				mockEntry := mocks.NewMockKeyValueEntry(s.mockCtrl)
				mockEntry.EXPECT().Value().Return([]byte(`{"status": "ok"}`))
				mockEntry.EXPECT().Revision().Return(uint64(1))

				s.mockKV.EXPECT().
					Get(gomock.Any()).
					Return(mockEntry, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:         "empty RequestID gets generated",
			subject:      "test.generate_id",
			data:         []byte(`{"test": "generate_id"}`),
			requestID:    "", // Will be generated
			responseData: []byte(`{"status": "ok"}`),
			mockSetup: func() {
				// Mock publish
				s.mockExt.EXPECT().
					PublishMsg(gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)

				// Mock KV get returns response immediately
				mockEntry := mocks.NewMockKeyValueEntry(s.mockCtrl)
				mockEntry.EXPECT().Value().Return([]byte(`{"status": "ok"}`))
				mockEntry.EXPECT().Revision().Return(uint64(1))

				s.mockKV.EXPECT().
					Get(gomock.Any()).
					Return(mockEntry, nil).
					Times(1)
			},
			expectedErr: "",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			ctx := context.Background()
			var opts *client.RequestReplyOptions

			// Handle special test cases
			if tc.name == "nil options uses defaults" {
				opts = nil // Test nil options path
			} else if tc.name == "empty RequestID gets generated" {
				opts = &client.RequestReplyOptions{
					Timeout:      100 * time.Millisecond,
					PollInterval: 10 * time.Millisecond,
					// RequestID left empty to test generation
				}
			} else {
				opts = &client.RequestReplyOptions{
					RequestID:    tc.requestID,
					Timeout:      100 * time.Millisecond,
					PollInterval: 10 * time.Millisecond,
				}

				// Handle timeout and cancellation test cases
				if tc.name == "timeout waiting for response" {
					// Use a very short timeout to trigger timeout quickly
					opts.Timeout = 20 * time.Millisecond
				} else if tc.name == "context cancelled during wait" {
					// Create a context that gets cancelled immediately
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					go func() {
						time.Sleep(5 * time.Millisecond) // Give publish time to complete
						cancel()
					}()
				}
			}

			response, err := s.client.PublishAndWaitKV(
				ctx,
				tc.subject,
				tc.data,
				s.mockKV,
				opts,
			)

			if tc.expectedErr == "" {
				s.NoError(err)
				s.Equal(tc.responseData, response)
			} else {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedErr)
			}
		})
	}
}

func (s *KVCreatePublicTestSuite) TestWatchKV() {
	tests := []struct {
		name          string
		pattern       string
		setupMocks    func() *mocks.MockKeyWatcher
		expectedError string
	}{
		{
			name:    "successfully creates watcher",
			pattern: "test.*",
			setupMocks: func() *mocks.MockKeyWatcher {
				mockWatcher := mocks.NewMockKeyWatcher(s.mockCtrl)

				// Set up channel for Updates() - the goroutine will call this immediately
				updatesChan := make(chan nats.KeyValueEntry, 1)
				close(updatesChan) // Close immediately so goroutine exits

				s.mockKV.EXPECT().
					Watch("test.*").
					Return(mockWatcher, nil)

				mockWatcher.EXPECT().
					Updates().
					Return(updatesChan).
					AnyTimes()

				mockWatcher.EXPECT().
					Stop().
					Return(nil).
					AnyTimes()

				return mockWatcher
			},
		},
		{
			name:          "error creating watcher",
			pattern:       "invalid.*",
			expectedError: "failed to create watcher",
			setupMocks: func() *mocks.MockKeyWatcher {
				s.mockKV.EXPECT().
					Watch("invalid.*").
					Return(nil, errors.New("watch error"))
				return nil
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			mockWatcher := tc.setupMocks()

			ctx := context.Background()
			ch, err := s.client.WatchKV(ctx, s.mockKV, tc.pattern)

			if tc.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedError)
				s.Nil(ch)
			} else {
				s.NoError(err)
				s.NotNil(ch)
				// For successful cases, just verify the channel was created
				// Testing the goroutine behavior would require complex setup
				_ = mockWatcher // Use the watcher to avoid unused variable
			}
		})
	}
}

func TestKVCreatePublicTestSuite(t *testing.T) {
	suite.Run(t, new(KVCreatePublicTestSuite))
}

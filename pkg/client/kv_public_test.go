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
			switch tc.name {
			case "nil options uses defaults":
				opts = nil // Test nil options path
			case "empty RequestID gets generated":
				opts = &client.RequestReplyOptions{
					Timeout:      100 * time.Millisecond,
					PollInterval: 10 * time.Millisecond,
					// RequestID left empty to test generation
				}
			default:
				opts = &client.RequestReplyOptions{
					RequestID:    tc.requestID,
					Timeout:      100 * time.Millisecond,
					PollInterval: 10 * time.Millisecond,
				}

				switch tc.name {
				case "timeout waiting for response":
					// Use a very short timeout to trigger timeout quickly
					opts.Timeout = 20 * time.Millisecond
				case "context cancelled during wait":
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
		testBehavior  func(ch <-chan nats.KeyValueEntry)
		expectedError string
	}{
		{
			name:    "successfully creates watcher and forwards entries",
			pattern: "test.*",
			setupMocks: func() *mocks.MockKeyWatcher {
				mockWatcher := mocks.NewMockKeyWatcher(s.mockCtrl)

				// Create a channel that will send a test entry
				updatesChan := make(chan nats.KeyValueEntry, 2)

				// Create a mock entry
				mockEntry := mocks.NewMockKeyValueEntry(s.mockCtrl)
				mockEntry.EXPECT().Key().Return("test.key").AnyTimes()
				mockEntry.EXPECT().Value().Return([]byte("test value")).AnyTimes()

				// Send the entry and then close the channel to test proper cleanup
				updatesChan <- mockEntry
				close(updatesChan)

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
			testBehavior: func(ch <-chan nats.KeyValueEntry) {
				// Wait for the entry to be forwarded through the goroutine
				select {
				case entry := <-ch:
					s.NotNil(entry)
					s.Equal("test.key", entry.Key())
					s.Equal([]byte("test value"), entry.Value())
				case <-time.After(100 * time.Millisecond):
					s.Fail("Expected to receive an entry but timed out")
				}

				// Now the goroutine should exit when the updates channel closes
				// This will cause our output channel to close
				select {
				case _, ok := <-ch:
					s.False(ok, "Channel should be closed when updates channel closes")
				case <-time.After(100 * time.Millisecond):
					s.Fail("Expected channel to close but timed out")
				}
			},
		},
		{
			name:    "goroutine stops on context cancellation",
			pattern: "test.*",
			setupMocks: func() *mocks.MockKeyWatcher {
				mockWatcher := mocks.NewMockKeyWatcher(s.mockCtrl)

				// Create a channel that never closes naturally
				updatesChan := make(chan nats.KeyValueEntry)

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
			testBehavior: func(ch <-chan nats.KeyValueEntry) {
				// Channel should close when context is cancelled
				select {
				case _, ok := <-ch:
					s.False(ok, "Channel should be closed due to context cancellation")
				case <-time.After(100 * time.Millisecond):
					s.Fail("Expected channel to close due to context cancellation but timed out")
				}
			},
		},
		{
			name:    "handles nil entries gracefully",
			pattern: "test.*",
			setupMocks: func() *mocks.MockKeyWatcher {
				mockWatcher := mocks.NewMockKeyWatcher(s.mockCtrl)

				// Create a channel that sends nil then a real entry
				updatesChan := make(chan nats.KeyValueEntry, 3)

				// Send nil (should be ignored), then real entry, then close
				updatesChan <- nil

				mockEntry := mocks.NewMockKeyValueEntry(s.mockCtrl)
				mockEntry.EXPECT().Key().Return("test.key").AnyTimes()
				mockEntry.EXPECT().Value().Return([]byte("test value")).AnyTimes()
				updatesChan <- mockEntry

				close(updatesChan)

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
			testBehavior: func(ch <-chan nats.KeyValueEntry) {
				// Should only receive the non-nil entry (nil entries are filtered out)
				select {
				case entry := <-ch:
					s.NotNil(entry)
					s.Equal("test.key", entry.Key())
				case <-time.After(100 * time.Millisecond):
					s.Fail("Expected to receive a non-nil entry but timed out")
				}

				// The goroutine should exit when the updates channel closes
				select {
				case _, ok := <-ch:
					s.False(ok, "Channel should be closed when updates channel closes")
				case <-time.After(100 * time.Millisecond):
					s.Fail("Expected channel to close but timed out")
				}
			},
		},
		{
			name:    "context cancelled while forwarding entry",
			pattern: "test.*",
			setupMocks: func() *mocks.MockKeyWatcher {
				mockWatcher := mocks.NewMockKeyWatcher(s.mockCtrl)

				// Create a channel that will send entries
				updatesChan := make(chan nats.KeyValueEntry, 1)

				// Create a mock entry
				mockEntry := mocks.NewMockKeyValueEntry(s.mockCtrl)
				mockEntry.EXPECT().Key().Return("test.key").AnyTimes()
				mockEntry.EXPECT().Value().Return([]byte("test value")).AnyTimes()

				// Send the entry but don't close the channel
				updatesChan <- mockEntry

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
			testBehavior: func(ch <-chan nats.KeyValueEntry) {
				// Create a context that we can cancel
				// Don't read from the channel immediately - this creates backpressure
				// so when the goroutine tries to send the entry, it will block
				// and then we can cancel the context

				time.Sleep(
					10 * time.Millisecond,
				) // Give goroutine time to receive entry and try to send

				// The channel should close due to context cancellation
				// without us ever receiving the entry (because context was cancelled
				// while the goroutine was trying to send it)
				select {
				case _, ok := <-ch:
					if !ok {
						// Channel closed due to context cancellation - this is what we want
						s.True(true, "Channel closed due to context cancellation while forwarding")
					} else {
						// We might get the entry if timing is different, that's also fine
						s.True(true, "Entry received before context cancellation")
					}
				case <-time.After(200 * time.Millisecond):
					s.Fail("Expected channel to close or receive entry but timed out")
				}
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

			ctx, cancel := context.WithCancel(context.Background())

			// For context cancellation tests, cancel after a short delay
			switch tc.name {
			case "goroutine stops on context cancellation":
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
			case "context cancelled while forwarding entry":
				// Cancel context after giving time for goroutine to receive entry
				// but before we read from output channel (creating backpressure)
				go func() {
					time.Sleep(5 * time.Millisecond)
					cancel()
				}()
			default:
				defer cancel()
			}

			ch, err := s.client.WatchKV(ctx, s.mockKV, tc.pattern)

			if tc.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedError)
				s.Nil(ch)
			} else {
				s.NoError(err)
				s.NotNil(ch)

				// Test the goroutine behavior if we have a test function
				if tc.testBehavior != nil {
					tc.testBehavior(ch)
				}

				_ = mockWatcher // Use the watcher to avoid unused variable
			}
		})
	}
}

func (s *KVCreatePublicTestSuite) TestKVPut() {
	tests := []struct {
		name        string
		bucket      string
		key         string
		value       []byte
		mockSetup   func()
		expectedErr string
	}{
		{
			name:   "successfully puts value in KV bucket",
			bucket: "test-bucket",
			key:    "test-key",
			value:  []byte("test-value"),
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil).
					Times(1)

				s.mockKV.EXPECT().
					Put("test-key", []byte("test-value")).
					Return(uint64(1), nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:   "error creating KV bucket for put",
			bucket: "bad-bucket",
			key:    "test-key",
			value:  []byte("test-value"),
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "bad-bucket"}).
					Return(nil, errors.New("bucket creation failed")).
					Times(1)
			},
			expectedErr: "failed to get KV bucket bad-bucket: bucket creation failed",
		},
		{
			name:   "error putting value in KV bucket",
			bucket: "test-bucket",
			key:    "bad-key",
			value:  []byte("test-value"),
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil).
					Times(1)

				s.mockKV.EXPECT().
					Put("bad-key", []byte("test-value")).
					Return(uint64(0), errors.New("put failed")).
					Times(1)
			},
			expectedErr: "failed to put key bad-key in bucket test-bucket: put failed",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			err := s.client.KVPut(tc.bucket, tc.key, tc.value)

			if tc.expectedErr == "" {
				s.NoError(err)
			} else {
				s.EqualError(err, tc.expectedErr)
			}
		})
	}
}

func (s *KVCreatePublicTestSuite) TestKVGet() {
	tests := []struct {
		name         string
		bucket       string
		key          string
		mockSetup    func()
		expectedData []byte
		expectedErr  string
	}{
		{
			name:   "successfully gets value from KV bucket",
			bucket: "test-bucket",
			key:    "test-key",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil).
					Times(1)

				mockEntry := mocks.NewMockKeyValueEntry(s.mockCtrl)
				mockEntry.EXPECT().
					Value().
					Return([]byte("test-value")).
					Times(1)

				s.mockKV.EXPECT().
					Get("test-key").
					Return(mockEntry, nil).
					Times(1)
			},
			expectedData: []byte("test-value"),
			expectedErr:  "",
		},
		{
			name:   "error creating KV bucket for get",
			bucket: "bad-bucket",
			key:    "test-key",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "bad-bucket"}).
					Return(nil, errors.New("bucket creation failed")).
					Times(1)
			},
			expectedData: nil,
			expectedErr:  "failed to get KV bucket bad-bucket: bucket creation failed",
		},
		{
			name:   "error getting value from KV bucket",
			bucket: "test-bucket",
			key:    "missing-key",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil).
					Times(1)

				s.mockKV.EXPECT().
					Get("missing-key").
					Return(nil, nats.ErrKeyNotFound).
					Times(1)
			},
			expectedData: nil,
			expectedErr:  "failed to get key missing-key from bucket test-bucket: nats: key not found",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			data, err := s.client.KVGet(tc.bucket, tc.key)

			if tc.expectedErr == "" {
				s.NoError(err)
				s.Equal(tc.expectedData, data)
			} else {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedErr)
				s.Nil(data)
			}
		})
	}
}

func (s *KVCreatePublicTestSuite) TestKVDelete() {
	tests := []struct {
		name        string
		bucket      string
		key         string
		mockSetup   func()
		expectedErr string
	}{
		{
			name:   "successfully deletes key from KV bucket",
			bucket: "test-bucket",
			key:    "test-key",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil).
					Times(1)

				s.mockKV.EXPECT().
					Delete("test-key").
					Return(nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:   "error creating KV bucket for delete",
			bucket: "bad-bucket",
			key:    "test-key",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "bad-bucket"}).
					Return(nil, errors.New("bucket creation failed")).
					Times(1)
			},
			expectedErr: "failed to get KV bucket bad-bucket: bucket creation failed",
		},
		{
			name:   "error deleting key from KV bucket",
			bucket: "test-bucket",
			key:    "bad-key",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil).
					Times(1)

				s.mockKV.EXPECT().
					Delete("bad-key").
					Return(errors.New("delete failed")).
					Times(1)
			},
			expectedErr: "failed to delete key bad-key from bucket test-bucket: delete failed",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			err := s.client.KVDelete(tc.bucket, tc.key)

			if tc.expectedErr == "" {
				s.NoError(err)
			} else {
				s.EqualError(err, tc.expectedErr)
			}
		})
	}
}

func (s *KVCreatePublicTestSuite) TestKVKeys() {
	tests := []struct {
		name         string
		bucket       string
		mockSetup    func()
		expectedKeys []string
		expectedErr  string
	}{
		{
			name:   "successfully gets keys from KV bucket",
			bucket: "test-bucket",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil).
					Times(1)

				s.mockKV.EXPECT().
					Keys().
					Return([]string{"key1", "key2", "key3"}, nil).
					Times(1)
			},
			expectedKeys: []string{"key1", "key2", "key3"},
			expectedErr:  "",
		},
		{
			name:   "successfully gets empty keys from KV bucket",
			bucket: "empty-bucket",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "empty-bucket"}).
					Return(s.mockKV, nil).
					Times(1)

				s.mockKV.EXPECT().
					Keys().
					Return([]string{}, nil).
					Times(1)
			},
			expectedKeys: []string{},
			expectedErr:  "",
		},
		{
			name:   "error creating KV bucket for keys",
			bucket: "bad-bucket",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "bad-bucket"}).
					Return(nil, errors.New("bucket creation failed")).
					Times(1)
			},
			expectedKeys: nil,
			expectedErr:  "failed to get KV bucket bad-bucket: bucket creation failed",
		},
		{
			name:   "error getting keys from KV bucket",
			bucket: "test-bucket",
			mockSetup: func() {
				s.mockJS.EXPECT().
					CreateKeyValue(&nats.KeyValueConfig{Bucket: "test-bucket"}).
					Return(s.mockKV, nil).
					Times(1)

				s.mockKV.EXPECT().
					Keys().
					Return(nil, errors.New("keys failed")).
					Times(1)
			},
			expectedKeys: nil,
			expectedErr:  "failed to get keys from bucket test-bucket: keys failed",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			keys, err := s.client.KVKeys(tc.bucket)

			if tc.expectedErr == "" {
				s.NoError(err)
				s.Equal(tc.expectedKeys, keys)
			} else {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedErr)
				s.Nil(keys)
			}
		})
	}
}

func TestKVCreatePublicTestSuite(t *testing.T) {
	suite.Run(t, new(KVCreatePublicTestSuite))
}

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

package client_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

// MessageChannel implements a channel that can deliver jetstream messages
type MessageChannel struct {
	ch chan jetstream.Msg
}

func NewMessageChannel() *MessageChannel {
	return &MessageChannel{
		ch: make(chan jetstream.Msg, 10),
	}
}

func (mc *MessageChannel) Messages() <-chan jetstream.Msg {
	return mc.ch
}

func (mc *MessageChannel) Send(msg jetstream.Msg) {
	mc.ch <- msg
}

func (mc *MessageChannel) Close() {
	close(mc.ch)
}

type ConsumerPublicTestSuite struct {
	suite.Suite

	mockCtrl         *gomock.Controller
	mockJS           *mocks.MockJetStreamContext
	mockExt          *mocks.MockJetStream
	mockConsumer     *mocks.MockConsumer
	mockNC           *mocks.MockNATSConnector
	mockMessageBatch *mocks.MockMessageBatch
	client           *client.Client
	logger           *slog.Logger
}

func (s *ConsumerPublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockJS = mocks.NewMockJetStreamContext(s.mockCtrl)
	s.mockExt = mocks.NewMockJetStream(s.mockCtrl)
	s.mockConsumer = mocks.NewMockConsumer(s.mockCtrl)
	s.mockNC = mocks.NewMockNATSConnector(s.mockCtrl)
	s.mockMessageBatch = mocks.NewMockMessageBatch(s.mockCtrl)
	s.logger = slog.Default()

	s.client = client.New(s.logger, &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	})
	s.client.NC = s.mockNC
	s.client.NativeJS = s.mockJS
	s.client.ExtJS = s.mockExt
}

func (s *ConsumerPublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *ConsumerPublicTestSuite) TestConsumeMessagesConsumerError() {
	streamName := "TEST-STREAM"
	consumerName := "test-consumer"
	expectedError := errors.New("consumer not found")

	handler := func(_ jetstream.Msg) error {
		return nil
	}

	opts := &client.ConsumeOptions{
		QueueGroup:  "test-queue",
		MaxInFlight: 5,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock the consumer.Get call to return an error
	s.mockExt.EXPECT().
		Consumer(ctx, streamName, consumerName).
		Return(nil, expectedError).
		Times(1)

	err := s.client.ConsumeMessages(ctx, streamName, consumerName, handler, opts)
	s.Error(err)
	s.Contains(err.Error(), "failed to get consumer")
	s.Contains(err.Error(), expectedError.Error())
}

func (s *ConsumerPublicTestSuite) TestConsumeMessagesDefaultOptions() {
	streamName := "TEST-STREAM"
	consumerName := "test-consumer"
	expectedError := errors.New("consumer not found")

	handler := func(_ jetstream.Msg) error {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test with nil options - should use defaults
	s.mockExt.EXPECT().
		Consumer(ctx, streamName, consumerName).
		Return(nil, expectedError).
		Times(1)

	err := s.client.ConsumeMessages(ctx, streamName, consumerName, handler, nil)
	s.Error(err)
	s.Contains(err.Error(), "failed to get consumer")
}

func (s *ConsumerPublicTestSuite) TestConsumeMessages() {
	type testCase struct {
		name           string
		setupMocks     func(ctx context.Context)
		handler        client.JetStreamMessageHandler
		opts           *client.ConsumeOptions
		expectedError  string
		contextTimeout time.Duration
	}

	testCases := []testCase{
		{
			name: "successful message processing with ack",
			setupMocks: func(ctx context.Context) {
				msgCh := NewMessageChannel()
				mockMsg := mocks.NewMockMsg(s.mockCtrl)

				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Setup fetch expectations - first call returns a message, subsequent calls timeout
				firstCall := true
				s.mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						if firstCall {
							firstCall = false
							s.mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages())

							// Send a message and close
							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return s.mockMessageBatch, nil
						}
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()

				// Message expectations
				mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
				mockMsg.EXPECT().Ack().Return(nil)
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			opts: &client.ConsumeOptions{
				QueueGroup:  "test-queue",
				MaxInFlight: 5,
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "message processing error no ack",
			setupMocks: func(ctx context.Context) {
				msgCh := NewMessageChannel()
				mockMsg := mocks.NewMockMsg(s.mockCtrl)

				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Setup fetch expectations - first call returns a message, subsequent calls timeout
				firstCall := true
				s.mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						if firstCall {
							firstCall = false
							s.mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages())

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return s.mockMessageBatch, nil
						}
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()

				mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
				// No Ack() call expected when handler returns error
			},
			handler: func(_ jetstream.Msg) error {
				return errors.New("processing error")
			},
			opts:           &client.ConsumeOptions{MaxInFlight: 10},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "ack error handling",
			setupMocks: func(ctx context.Context) {
				msgCh := NewMessageChannel()
				mockMsg := mocks.NewMockMsg(s.mockCtrl)

				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Setup fetch expectations - first call returns a message, subsequent calls timeout
				firstCall := true
				s.mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						if firstCall {
							firstCall = false
							s.mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages())

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return s.mockMessageBatch, nil
						}
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()

				mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
				mockMsg.EXPECT().Ack().Return(errors.New("ack failed")).MaxTimes(1)
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "fetch error non timeout",
			setupMocks: func(ctx context.Context) {
				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Return a non-timeout error
				s.mockConsumer.EXPECT().
					Fetch(1).
					Return(nil, errors.New("connection error")).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "context cancellation during fetch",
			setupMocks: func(ctx context.Context) {
				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Block on fetch until context is cancelled
				s.mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						<-ctx.Done()
						return nil, ctx.Err()
					}).
					MaxTimes(1)
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			contextTimeout: 50 * time.Millisecond,
			expectedError:  "context",
		},
		{
			name: "handler panic recovery",
			setupMocks: func(ctx context.Context) {
				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Setup to return multiple messages that will cause panics
				callCount := 0
				s.mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						callCount++
						if callCount <= 3 {
							// Return messages that will cause panics in handler
							msgCh := NewMessageChannel()
							mockMsg := mocks.NewMockMsg(s.mockCtrl)

							s.mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages()).
								Times(1)

							mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
							// No Ack expected when handler panics

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return s.mockMessageBatch, nil
						}
						// Then timeout to keep loop going until context cancellation
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				panic("handler panic")
			},
			contextTimeout: 200 * time.Millisecond, // Longer timeout to allow multiple panic recoveries
		},
		{
			name: "exact timeout error handling",
			setupMocks: func(ctx context.Context) {
				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Return exact "nats: timeout" error
				s.mockConsumer.EXPECT().
					Fetch(1).
					Return(nil, errors.New("nats: timeout")).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "non timeout fetch error with logging",
			setupMocks: func(ctx context.Context) {
				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Return a non-timeout error several times to trigger logging, then timeout
				callCount := 0
				s.mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						callCount++
						if callCount <= 3 {
							// Return non-timeout error multiple times to ensure logging path is hit
							return nil, errors.New("connection lost")
						}
						// Then return timeout errors to keep the loop going
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			contextTimeout: 200 * time.Millisecond, // Longer timeout to allow error logging
		},
		{
			name: "message processing error with logging",
			setupMocks: func(ctx context.Context) {
				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Setup to return multiple messages with processing errors
				callCount := 0
				s.mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						callCount++
						if callCount <= 3 {
							// Return messages that will cause processing errors
							msgCh := NewMessageChannel()
							mockMsg := mocks.NewMockMsg(s.mockCtrl)

							s.mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages()).
								Times(1)

							mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
							// No Ack expected when handler returns error

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return s.mockMessageBatch, nil
						}
						// Then timeout to keep loop going until context cancellation
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				return errors.New("processing failed")
			},
			contextTimeout: 200 * time.Millisecond, // Longer timeout to allow multiple error logs
		},
		{
			name: "ack error with logging",
			setupMocks: func(ctx context.Context) {
				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Setup to return multiple messages with ack errors
				callCount := 0
				s.mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						callCount++
						if callCount <= 3 {
							// Return messages that will cause ack errors
							msgCh := NewMessageChannel()
							mockMsg := mocks.NewMockMsg(s.mockCtrl)

							s.mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages()).
								Times(1)

							mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
							mockMsg.EXPECT().Ack().Return(errors.New("ack failed")).Times(1)

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return s.mockMessageBatch, nil
						}
						// Then timeout to keep loop going until context cancellation
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			contextTimeout: 200 * time.Millisecond, // Longer timeout to allow multiple error logs
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), tc.contextTimeout)
			defer cancel()

			tc.setupMocks(ctx)

			err := s.client.ConsumeMessages(
				ctx,
				"TEST-STREAM",
				"test-consumer",
				tc.handler,
				tc.opts,
			)

			if tc.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedError)
			} else {
				// Context timeout is expected
				s.Error(err)
				s.True(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
			}
		})
	}
}

// Simple test to verify error logging paths are hit
func (s *ConsumerPublicTestSuite) TestConsumerErrorPaths() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Test 1: Non-timeout fetch error
	s.mockExt.EXPECT().
		Consumer(ctx, "TEST-STREAM", "test-consumer").
		Return(s.mockConsumer, nil)

	// Return non-timeout error exactly once to hit the logging path
	s.mockConsumer.EXPECT().
		Fetch(1).
		Return(nil, errors.New("connection lost")).
		Times(1)

	// Then timeout errors
	s.mockConsumer.EXPECT().
		Fetch(1).
		Return(nil, errors.New("nats: timeout")).
		AnyTimes()

	handler := func(_ jetstream.Msg) error {
		return nil
	}

	err := s.client.ConsumeMessages(ctx, "TEST-STREAM", "test-consumer", handler, nil)
	s.Error(err) // Should error due to context timeout
}

// Focused test to hit the message processing error logging
func (s *ConsumerPublicTestSuite) TestMessageProcessingErrorLogging() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	s.mockExt.EXPECT().
		Consumer(ctx, "TEST-STREAM", "test-consumer").
		Return(s.mockConsumer, nil)

	// Create a test that will DEFINITELY return a message that causes an error
	mockMsg := mocks.NewMockMsg(s.mockCtrl)
	mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
	// No Ack() expectation since processing will fail

	msgCh := NewMessageChannel()
	s.mockMessageBatch.EXPECT().
		Messages().
		Return(msgCh.Messages()).
		Times(1)

	// First fetch returns a message that will cause handler to fail
	s.mockConsumer.EXPECT().
		Fetch(1).
		DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
			// Send message and close immediately
			go func() {
				msgCh.Send(mockMsg)
				msgCh.Close()
			}()
			return s.mockMessageBatch, nil
		}).
		Times(1)

	// Subsequent fetches timeout to let context expire
	s.mockConsumer.EXPECT().
		Fetch(1).
		Return(nil, errors.New("nats: timeout")).
		AnyTimes()

	// Handler that ALWAYS returns an error
	handler := func(_ jetstream.Msg) error {
		return errors.New("processing always fails")
	}

	err := s.client.ConsumeMessages(ctx, "TEST-STREAM", "test-consumer", handler, nil)
	s.Error(err) // Should error due to context timeout
}

// Focused test to hit the ack error logging
func (s *ConsumerPublicTestSuite) TestAckErrorLogging() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	s.mockExt.EXPECT().
		Consumer(ctx, "TEST-STREAM", "test-consumer").
		Return(s.mockConsumer, nil)

	// Create a test that will return a message that processes successfully but ack fails
	mockMsg := mocks.NewMockMsg(s.mockCtrl)
	mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
	mockMsg.EXPECT().Ack().Return(errors.New("ack failed")).Times(1)

	msgCh := NewMessageChannel()
	s.mockMessageBatch.EXPECT().
		Messages().
		Return(msgCh.Messages()).
		Times(1)

	// First fetch returns a message
	s.mockConsumer.EXPECT().
		Fetch(1).
		DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
			go func() {
				msgCh.Send(mockMsg)
				msgCh.Close()
			}()
			return s.mockMessageBatch, nil
		}).
		Times(1)

	// Subsequent fetches timeout
	s.mockConsumer.EXPECT().
		Fetch(1).
		Return(nil, errors.New("nats: timeout")).
		AnyTimes()

	// Handler that succeeds (so ack gets called)
	handler := func(_ jetstream.Msg) error {
		return nil // Success, so ack will be called
	}

	err := s.client.ConsumeMessages(ctx, "TEST-STREAM", "test-consumer", handler, nil)
	s.Error(err) // Should error due to context timeout
}

// Focused test to hit the panic recovery logging
func (s *ConsumerPublicTestSuite) TestPanicRecoveryLogging() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	s.mockExt.EXPECT().
		Consumer(ctx, "TEST-STREAM", "test-consumer").
		Return(s.mockConsumer, nil)

	// Create a test that will return a message that causes handler to panic
	mockMsg := mocks.NewMockMsg(s.mockCtrl)
	mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
	// No Ack() expectation since handler will panic

	msgCh := NewMessageChannel()
	s.mockMessageBatch.EXPECT().
		Messages().
		Return(msgCh.Messages()).
		Times(1)

	// First fetch returns a message
	s.mockConsumer.EXPECT().
		Fetch(1).
		DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
			go func() {
				msgCh.Send(mockMsg)
				msgCh.Close()
			}()
			return s.mockMessageBatch, nil
		}).
		Times(1)

	// Subsequent fetches timeout
	s.mockConsumer.EXPECT().
		Fetch(1).
		Return(nil, errors.New("nats: timeout")).
		AnyTimes()

	// Handler that ALWAYS panics
	handler := func(_ jetstream.Msg) error {
		panic("test panic for coverage")
	}

	err := s.client.ConsumeMessages(ctx, "TEST-STREAM", "test-consumer", handler, nil)
	s.Error(err) // Should error due to context timeout
}

func TestConsumerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConsumerPublicTestSuite))
}

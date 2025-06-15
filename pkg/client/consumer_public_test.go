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

func (s *ConsumerPublicTestSuite) TestConsumeMessages_ConsumerError() {
	streamName := "TEST-STREAM"
	consumerName := "test-consumer"
	expectedError := errors.New("consumer not found")

	handler := func(msg jetstream.Msg) error {
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

func (s *ConsumerPublicTestSuite) TestConsumeMessages_DefaultOptions() {
	streamName := "TEST-STREAM"
	consumerName := "test-consumer"
	expectedError := errors.New("consumer not found")

	handler := func(msg jetstream.Msg) error {
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

func (s *ConsumerPublicTestSuite) TestConsumeMessages_TableDriven() {
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
			name: "successful_message_processing_with_ack",
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
					DoAndReturn(func(n int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
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
			handler: func(msg jetstream.Msg) error {
				return nil
			},
			opts: &client.ConsumeOptions{
				QueueGroup:  "test-queue",
				MaxInFlight: 5,
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "message_processing_error_no_ack",
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
					DoAndReturn(func(n int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
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
			handler: func(msg jetstream.Msg) error {
				return errors.New("processing error")
			},
			opts:           &client.ConsumeOptions{MaxInFlight: 10},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "ack_error_handling",
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
					DoAndReturn(func(n int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
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
			handler: func(msg jetstream.Msg) error {
				return nil
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "fetch_error_non_timeout",
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
			handler: func(msg jetstream.Msg) error {
				return nil
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "context_cancellation_during_fetch",
			setupMocks: func(ctx context.Context) {
				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Block on fetch until context is cancelled
				s.mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(n int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						<-ctx.Done()
						return nil, ctx.Err()
					}).
					MaxTimes(1)
			},
			handler: func(msg jetstream.Msg) error {
				return nil
			},
			contextTimeout: 50 * time.Millisecond,
			expectedError:  "context",
		},
		{
			name: "handler_panic_recovery",
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
					DoAndReturn(func(n int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
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
				// No Ack expected when handler panics
			},
			handler: func(msg jetstream.Msg) error {
				panic("handler panic")
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "exact_timeout_error_handling",
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
			handler: func(msg jetstream.Msg) error {
				return nil
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "non_timeout_fetch_error_with_logging",
			setupMocks: func(ctx context.Context) {
				s.mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(s.mockConsumer, nil)

				// Return a non-timeout error to trigger logging
				s.mockConsumer.EXPECT().
					Fetch(1).
					Return(nil, errors.New("connection lost")).
					AnyTimes()
			},
			handler: func(msg jetstream.Msg) error {
				return nil
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "message_processing_error_with_logging",
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
					DoAndReturn(func(n int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
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
				// No Ack expected when handler returns error
			},
			handler: func(msg jetstream.Msg) error {
				return errors.New("processing failed")
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "ack_error_with_logging",
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
					DoAndReturn(func(n int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
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
			handler: func(msg jetstream.Msg) error {
				return nil
			},
			contextTimeout: 100 * time.Millisecond,
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

func TestConsumerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConsumerPublicTestSuite))
}

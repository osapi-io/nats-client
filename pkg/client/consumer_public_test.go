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

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

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
}

// resetMocks creates fresh mock objects for each subtest, avoiding shared-state
// issues when gomock expectations (Times, AnyTimes) span subtests.
func (s *ConsumerPublicTestSuite) resetMocks() (
	*gomock.Controller,
	*mocks.MockJetStream,
	*mocks.MockConsumer,
	*mocks.MockMessageBatch,
	*client.Client,
) {
	ctrl := gomock.NewController(s.T())
	mockExt := mocks.NewMockJetStream(ctrl)
	mockConsumer := mocks.NewMockConsumer(ctrl)
	mockNC := mocks.NewMockNATSConnector(ctrl)
	mockMessageBatch := mocks.NewMockMessageBatch(ctrl)
	logger := slog.Default()

	c := client.New(logger, &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	})
	c.NC = mockNC
	c.ExtJS = mockExt

	return ctrl, mockExt, mockConsumer, mockMessageBatch, c
}

func (s *ConsumerPublicTestSuite) TestConsumeMessages() {
	type testCase struct {
		name           string
		setupMocks     func(ctx context.Context, ctrl *gomock.Controller, mockExt *mocks.MockJetStream, mockConsumer *mocks.MockConsumer, mockMessageBatch *mocks.MockMessageBatch)
		handler        client.JetStreamMessageHandler
		opts           *client.ConsumeOptions
		expectedError  string
		contextTimeout time.Duration
		assertFn       func(err error)
	}

	testCases := []testCase{
		{
			name: "consumer error",
			setupMocks: func(
				ctx context.Context,
				_ *gomock.Controller,
				mockExt *mocks.MockJetStream,
				_ *mocks.MockConsumer,
				_ *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(nil, errors.New("consumer not found")).
					Times(1)
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			opts: &client.ConsumeOptions{
				QueueGroup:  "test-queue",
				MaxInFlight: 5,
			},
			contextTimeout: 100 * time.Millisecond,
			assertFn: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "failed to get consumer")
				s.Contains(err.Error(), "consumer not found")
			},
		},
		{
			name: "consumer error with default options",
			setupMocks: func(
				ctx context.Context,
				_ *gomock.Controller,
				mockExt *mocks.MockJetStream,
				_ *mocks.MockConsumer,
				_ *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(nil, errors.New("consumer not found")).
					Times(1)
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			opts:           nil,
			contextTimeout: 100 * time.Millisecond,
			assertFn: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "failed to get consumer")
			},
		},
		{
			name: "successful message processing with ack",
			setupMocks: func(
				ctx context.Context,
				ctrl *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				mockMessageBatch *mocks.MockMessageBatch,
			) {
				msgCh := NewMessageChannel()
				mockMsg := mocks.NewMockMsg(ctrl)

				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				firstCall := true
				mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						if firstCall {
							firstCall = false
							mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages())

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return mockMessageBatch, nil
						}
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()

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
			setupMocks: func(
				ctx context.Context,
				ctrl *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				mockMessageBatch *mocks.MockMessageBatch,
			) {
				msgCh := NewMessageChannel()
				mockMsg := mocks.NewMockMsg(ctrl)

				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				firstCall := true
				mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						if firstCall {
							firstCall = false
							mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages())

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return mockMessageBatch, nil
						}
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()

				mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				return errors.New("processing error")
			},
			opts:           &client.ConsumeOptions{MaxInFlight: 10},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "ack error handling",
			setupMocks: func(
				ctx context.Context,
				ctrl *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				mockMessageBatch *mocks.MockMessageBatch,
			) {
				msgCh := NewMessageChannel()
				mockMsg := mocks.NewMockMsg(ctrl)

				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				firstCall := true
				mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						if firstCall {
							firstCall = false
							mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages())

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return mockMessageBatch, nil
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
			setupMocks: func(
				ctx context.Context,
				_ *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				_ *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				mockConsumer.EXPECT().
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
			setupMocks: func(
				ctx context.Context,
				_ *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				_ *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				mockConsumer.EXPECT().
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
			setupMocks: func(
				ctx context.Context,
				ctrl *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				mockMessageBatch *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				callCount := 0
				mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						callCount++
						if callCount <= 3 {
							msgCh := NewMessageChannel()
							mockMsg := mocks.NewMockMsg(ctrl)

							mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages()).
								Times(1)

							mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return mockMessageBatch, nil
						}
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				panic("handler panic")
			},
			contextTimeout: 200 * time.Millisecond,
		},
		{
			name: "exact timeout error handling",
			setupMocks: func(
				ctx context.Context,
				_ *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				_ *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				mockConsumer.EXPECT().
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
			setupMocks: func(
				ctx context.Context,
				_ *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				_ *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				callCount := 0
				mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						callCount++
						if callCount <= 3 {
							return nil, errors.New("connection lost")
						}
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			contextTimeout: 200 * time.Millisecond,
		},
		{
			name: "non timeout fetch error with ordered expectations",
			setupMocks: func(
				ctx context.Context,
				_ *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				_ *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				s.T().Helper()
				gomock.InOrder(
					mockConsumer.EXPECT().
						Fetch(1).
						Return(nil, errors.New("connection lost")).
						Times(1),
					mockConsumer.EXPECT().
						Fetch(1).
						Return(nil, errors.New("nats: timeout")).
						AnyTimes(),
				)
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			opts:           nil,
			contextTimeout: 50 * time.Millisecond,
		},
		{
			name: "message processing error with logging",
			setupMocks: func(
				ctx context.Context,
				ctrl *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				mockMessageBatch *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				callCount := 0
				mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						callCount++
						if callCount <= 3 {
							msgCh := NewMessageChannel()
							mockMsg := mocks.NewMockMsg(ctrl)

							mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages()).
								Times(1)

							mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return mockMessageBatch, nil
						}
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				return errors.New("processing failed")
			},
			contextTimeout: 200 * time.Millisecond,
		},
		{
			name: "focused message processing error logging",
			setupMocks: func(
				ctx context.Context,
				ctrl *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				mockMessageBatch *mocks.MockMessageBatch,
			) {
				mockMsg := mocks.NewMockMsg(ctrl)
				mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()

				msgCh := NewMessageChannel()
				mockMessageBatch.EXPECT().
					Messages().
					Return(msgCh.Messages()).
					Times(1)

				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				gomock.InOrder(
					mockConsumer.EXPECT().
						Fetch(1).
						DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()
							return mockMessageBatch, nil
						}).
						Times(1),
					mockConsumer.EXPECT().
						Fetch(1).
						Return(nil, errors.New("nats: timeout")).
						AnyTimes(),
				)
			},
			handler: func(_ jetstream.Msg) error {
				return errors.New("processing always fails")
			},
			opts:           nil,
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "focused ack error logging",
			setupMocks: func(
				ctx context.Context,
				ctrl *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				mockMessageBatch *mocks.MockMessageBatch,
			) {
				mockMsg := mocks.NewMockMsg(ctrl)
				mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
				mockMsg.EXPECT().Ack().Return(errors.New("ack failed")).Times(1)

				msgCh := NewMessageChannel()
				mockMessageBatch.EXPECT().
					Messages().
					Return(msgCh.Messages()).
					Times(1)

				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				gomock.InOrder(
					mockConsumer.EXPECT().
						Fetch(1).
						DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()
							return mockMessageBatch, nil
						}).
						Times(1),
					mockConsumer.EXPECT().
						Fetch(1).
						Return(nil, errors.New("nats: timeout")).
						AnyTimes(),
				)
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			opts:           nil,
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "focused panic recovery logging",
			setupMocks: func(
				ctx context.Context,
				ctrl *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				mockMessageBatch *mocks.MockMessageBatch,
			) {
				mockMsg := mocks.NewMockMsg(ctrl)
				mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()

				msgCh := NewMessageChannel()
				mockMessageBatch.EXPECT().
					Messages().
					Return(msgCh.Messages()).
					Times(1)

				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				gomock.InOrder(
					mockConsumer.EXPECT().
						Fetch(1).
						DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()
							return mockMessageBatch, nil
						}).
						Times(1),
					mockConsumer.EXPECT().
						Fetch(1).
						Return(nil, errors.New("nats: timeout")).
						AnyTimes(),
				)
			},
			handler: func(_ jetstream.Msg) error {
				panic("test panic for coverage")
			},
			opts:           nil,
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "ack error with logging",
			setupMocks: func(
				ctx context.Context,
				ctrl *gomock.Controller,
				mockExt *mocks.MockJetStream,
				mockConsumer *mocks.MockConsumer,
				mockMessageBatch *mocks.MockMessageBatch,
			) {
				mockExt.EXPECT().
					Consumer(ctx, "TEST-STREAM", "test-consumer").
					Return(mockConsumer, nil)

				callCount := 0
				mockConsumer.EXPECT().
					Fetch(1).
					DoAndReturn(func(_ int, _ ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
						callCount++
						if callCount <= 3 {
							msgCh := NewMessageChannel()
							mockMsg := mocks.NewMockMsg(ctrl)

							mockMessageBatch.EXPECT().
								Messages().
								Return(msgCh.Messages()).
								Times(1)

							mockMsg.EXPECT().Subject().Return("test.subject").AnyTimes()
							mockMsg.EXPECT().Ack().Return(errors.New("ack failed")).Times(1)

							go func() {
								msgCh.Send(mockMsg)
								msgCh.Close()
							}()

							return mockMessageBatch, nil
						}
						return nil, errors.New("nats: timeout")
					}).
					AnyTimes()
			},
			handler: func(_ jetstream.Msg) error {
				return nil
			},
			contextTimeout: 200 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ctrl, mockExt, mockConsumer, mockMessageBatch, c := s.resetMocks()
			defer ctrl.Finish()

			ctx, cancel := context.WithTimeout(context.Background(), tc.contextTimeout)
			defer cancel()

			tc.setupMocks(ctx, ctrl, mockExt, mockConsumer, mockMessageBatch)

			err := c.ConsumeMessages(
				ctx,
				"TEST-STREAM",
				"test-consumer",
				tc.handler,
				tc.opts,
			)

			if tc.assertFn != nil {
				tc.assertFn(err)
			} else if tc.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tc.expectedError)
			} else {
				s.Error(err)
				s.True(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
			}
		})
	}
}

func TestConsumerPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConsumerPublicTestSuite))
}

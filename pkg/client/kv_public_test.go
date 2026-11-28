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
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

var errBoom = errors.New("boom")

// errStorageType is the error NATS returns for a change of the storage type
// of a bucket or a stream.
var errStorageType = &jetstream.APIError{ErrorCode: 10052, Description: "storage type change"}

type KVPublicTestSuite struct {
	suite.Suite

	ctrl   *gomock.Controller
	js     *mocks.MockJetStream
	kv     *mocks.MockKeyValue
	client *client.Client
	ctx    context.Context
}

func (s *KVPublicTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.js = mocks.NewMockJetStream(s.ctrl)
	s.kv = mocks.NewMockKeyValue(s.ctrl)
	s.client = client.New(slog.Default(), &client.Options{})
	client.SetJetStream(s.client, s.js)
	s.ctx = context.Background()
}

func (s *KVPublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

// open makes the server return the handle of the bucket "b" once.
func (s *KVPublicTestSuite) open() {
	s.js.EXPECT().KeyValue(gomock.Any(), "b").Return(s.kv, nil)
}

func (s *KVPublicTestSuite) TestKVCreateOrUpdate() {
	cfg := jetstream.KeyValueConfig{Bucket: "b"}

	tests := []struct {
		name    string
		setup   func()
		wantErr string
	}{
		{
			name: "when the bucket is made keeps its handle",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateKeyValue(gomock.Any(), cfg).Return(s.kv, nil)
			},
		},
		{
			name: "when only the storage type differs keeps the bucket",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateKeyValue(gomock.Any(), cfg).Return(nil, errStorageType)
				s.js.EXPECT().KeyValue(gomock.Any(), "b").Return(s.kv, nil)
			},
		},
		{
			name: "when the bucket that is kept cannot be read returns the error",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateKeyValue(gomock.Any(), cfg).Return(nil, errStorageType)
				s.js.EXPECT().KeyValue(gomock.Any(), "b").Return(nil, errBoom)
			},
			wantErr: "failed to create/update KV bucket b: boom",
		},
		{
			name: "when the server refuses returns the error",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateKeyValue(gomock.Any(), cfg).Return(nil, errBoom)
			},
			wantErr: "failed to create/update KV bucket b: boom",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.setup()

			err := s.client.KVCreateOrUpdate(s.ctx, cfg)
			if tc.wantErr != "" {
				s.EqualError(err, tc.wantErr)
				return
			}
			s.Require().NoError(err)

			// The next call uses the handle, and does not ask the server.
			s.kv.EXPECT().Put(gomock.Any(), "k", []byte("v")).Return(uint64(1), nil)
			_, err = s.client.KVPut(s.ctx, "b", "k", []byte("v"))
			s.NoError(err)
		})
	}
}

func (s *KVPublicTestSuite) TestHandle() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "when the bucket is not there returns the error",
			validateFunc: func() {
				s.js.EXPECT().KeyValue(gomock.Any(), "b").Return(nil, jetstream.ErrBucketNotFound)

				_, err := s.client.KVGet(s.ctx, "b", "k")
				s.ErrorIs(err, jetstream.ErrBucketNotFound)
				s.ErrorContains(err, "failed to get KV bucket b")
			},
		},
		{
			name: "asks the server for the handle once",
			validateFunc: func() {
				s.open()
				s.kv.EXPECT().Delete(gomock.Any(), "k").Return(nil).Times(2)

				s.NoError(s.client.KVDelete(s.ctx, "b", "k"))
				s.NoError(s.client.KVDelete(s.ctx, "b", "k"))
			},
		},
		{
			name: "keeps no handle of a connection that changed during the call",
			validateFunc: func() {
				next := mocks.NewMockJetStream(s.ctrl)
				// The connection closes and connects again while the server
				// answers on the old one.
				s.js.EXPECT().KeyValue(gomock.Any(), "b").
					DoAndReturn(func(context.Context, string) (jetstream.KeyValue, error) {
						client.SetJetStream(s.client, next)
						return s.kv, nil
					})
				fresh := mocks.NewMockKeyValue(s.ctrl)
				next.EXPECT().KeyValue(gomock.Any(), "b").Return(fresh, nil)
				s.kv.EXPECT().Delete(gomock.Any(), "k").Return(nil)
				fresh.EXPECT().Delete(gomock.Any(), "k").Return(nil)

				s.NoError(s.client.KVDelete(s.ctx, "b", "k"))
				s.NoError(s.client.KVDelete(s.ctx, "b", "k"))
			},
		},
		{
			name: "asks again after a call says the connection closed",
			validateFunc: func() {
				s.js.EXPECT().KeyValue(gomock.Any(), "b").Return(s.kv, nil).Times(2)
				s.kv.EXPECT().Delete(gomock.Any(), "k").Return(nats.ErrConnectionClosed)
				s.kv.EXPECT().Delete(gomock.Any(), "k").Return(nil)

				s.ErrorIs(s.client.KVDelete(s.ctx, "b", "k"), nats.ErrConnectionClosed)
				s.NoError(s.client.KVDelete(s.ctx, "b", "k"))
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *KVPublicTestSuite) TestCalls() {
	entry := mocks.NewMockKeyValueEntry(gomock.NewController(s.T()))

	tests := []struct {
		name  string
		setup func(*mocks.MockKeyValue, error)
		call  func(*client.Client) (any, error)
		want  any
	}{
		{
			name: "KVGet",
			setup: func(kv *mocks.MockKeyValue, err error) {
				kv.EXPECT().Get(gomock.Any(), "k").Return(entry, err)
			},
			call: func(c *client.Client) (any, error) { return c.KVGet(s.ctx, "b", "k") },
			want: entry,
		},
		{
			name: "KVPut",
			setup: func(kv *mocks.MockKeyValue, err error) {
				kv.EXPECT().Put(gomock.Any(), "k", []byte("v")).Return(uint64(2), err)
			},
			call: func(c *client.Client) (any, error) { return c.KVPut(s.ctx, "b", "k", []byte("v")) },
			want: uint64(2),
		},
		{
			name: "KVCreate",
			setup: func(kv *mocks.MockKeyValue, err error) {
				kv.EXPECT().Create(gomock.Any(), "k", []byte("v")).Return(uint64(3), err)
			},
			call: func(c *client.Client) (any, error) { return c.KVCreate(s.ctx, "b", "k", []byte("v")) },
			want: uint64(3),
		},
		{
			name: "KVUpdate",
			setup: func(kv *mocks.MockKeyValue, err error) {
				kv.EXPECT().Update(gomock.Any(), "k", []byte("v"), uint64(3)).Return(uint64(4), err)
			},
			call: func(c *client.Client) (any, error) {
				return c.KVUpdate(s.ctx, "b", "k", []byte("v"), 3)
			},
			want: uint64(4),
		},
		{
			name: "KVDelete",
			setup: func(kv *mocks.MockKeyValue, err error) {
				kv.EXPECT().Delete(gomock.Any(), "k").Return(err)
			},
			call: func(c *client.Client) (any, error) { return nil, c.KVDelete(s.ctx, "b", "k") },
		},
		{
			name: "KVStatus",
			setup: func(kv *mocks.MockKeyValue, err error) {
				kv.EXPECT().Status(gomock.Any()).Return(nil, err)
			},
			call: func(c *client.Client) (any, error) {
				st, err := c.KVStatus(s.ctx, "b")
				if st == nil {
					return nil, err
				}

				return st, err
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name+" returns the answer of the bucket", func() {
			s.open()
			tc.setup(s.kv, nil)

			got, err := tc.call(s.client)
			s.NoError(err)
			s.Equal(tc.want, got)
		})
		s.Run(tc.name+" returns the error of the bucket", func() {
			s.open()
			tc.setup(s.kv, errBoom)

			_, err := tc.call(s.client)
			s.ErrorIs(err, errBoom)
		})
	}
}

// keyWatcher is a watcher of keys that sends an entry for each key, then
// the nil entry that marks the end of the keys when end is true, or closes
// its channel before that mark when end is false.
func (s *KVPublicTestSuite) keyWatcher(
	end bool,
	keys ...string,
) *mocks.MockKeyWatcher {
	ch := make(chan jetstream.KeyValueEntry, len(keys)+1)
	for _, k := range keys {
		e := mocks.NewMockKeyValueEntry(s.ctrl)
		e.EXPECT().Key().Return(k)
		ch <- e
	}
	if end {
		ch <- nil
	} else {
		close(ch)
	}
	w := mocks.NewMockKeyWatcher(s.ctrl)
	w.EXPECT().Updates().Return(ch).AnyTimes()
	w.EXPECT().Stop().Return(nil)

	return w
}

func (s *KVPublicTestSuite) TestKVKeys() {
	tests := []struct {
		name         string
		filters      []string
		ctx          func() context.Context
		setup        func()
		validateFunc func([]string, error)
	}{
		{
			name: "with no filter returns each key",
			setup: func() {
				s.kv.EXPECT().
					WatchAll(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(s.keyWatcher(true, "a", "b"), nil)
			},
			validateFunc: func(got []string, err error) {
				s.Require().NoError(err)
				s.Equal([]string{"a", "b"}, got)
			},
		},
		{
			name:    "with filters returns the keys that match",
			filters: []string{"jobs.>", "status.>"},
			setup: func() {
				s.kv.EXPECT().
					WatchFiltered(
						gomock.Any(),
						[]string{"jobs.>", "status.>"},
						gomock.Any(),
						gomock.Any(),
					).
					Return(s.keyWatcher(true, "jobs.1"), nil)
			},
			validateFunc: func(got []string, err error) {
				s.Require().NoError(err)
				s.Equal([]string{"jobs.1"}, got)
			},
		},
		{
			name: "when the bucket is empty returns an empty list",
			setup: func() {
				s.kv.EXPECT().
					WatchAll(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(s.keyWatcher(true), nil)
			},
			validateFunc: func(got []string, err error) {
				s.Require().NoError(err)
				s.Equal([]string{}, got)
			},
		},
		{
			name: "when the list fails returns the error",
			setup: func() {
				s.kv.EXPECT().
					WatchAll(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errBoom)
			},
			validateFunc: func(_ []string, err error) {
				s.ErrorIs(err, errBoom)
			},
		},
		{
			name: "when the watch stops before the end returns ErrWatchStopped, not a short list",
			setup: func() {
				s.kv.EXPECT().
					WatchAll(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(s.keyWatcher(false, "a"), nil)
			},
			validateFunc: func(got []string, err error) {
				s.ErrorIs(err, client.ErrWatchStopped)
				s.EqualError(
					err,
					"list the keys of b: the watch stopped before the end of the keys",
				)
				s.Nil(got)
			},
		},
		{
			name: "when ctx ends and the watch closes its channel returns the error of ctx",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(s.ctx)
				cancel()

				return ctx
			},
			setup: func() {
				s.kv.EXPECT().
					WatchAll(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(s.keyWatcher(false), nil)
			},
			validateFunc: func(got []string, err error) {
				s.ErrorIs(err, context.Canceled)
				s.NotErrorIs(err, client.ErrWatchStopped)
				s.Nil(got)
			},
		},
		{
			name: "when ctx ends during the list returns its error, not a short list",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(s.ctx)
				cancel()

				return ctx
			},
			setup: func() {
				w := mocks.NewMockKeyWatcher(s.ctrl)
				w.EXPECT().Updates().Return(make(chan jetstream.KeyValueEntry))
				w.EXPECT().Stop().Return(nil)
				s.kv.EXPECT().
					WatchAll(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(w, nil)
			},
			validateFunc: func(got []string, err error) {
				s.ErrorIs(err, context.Canceled)
				s.EqualError(err, "list the keys of b: context canceled")
				s.Nil(got)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.open()
			tc.setup()
			ctx := s.ctx
			if tc.ctx != nil {
				ctx = tc.ctx()
			}

			tc.validateFunc(s.client.KVKeys(ctx, "b", tc.filters...))
		})
	}
}

func (s *KVPublicTestSuite) TestKVWatch() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "when the watch cannot start returns the error",
			validateFunc: func() {
				s.open()
				s.kv.EXPECT().Watch(gomock.Any(), "jobs.>").Return(nil, errBoom)

				ch, err := s.client.KVWatch(s.ctx, "b", "jobs.>")
				s.ErrorIs(err, errBoom)
				s.Nil(ch)
			},
		},
		{
			name: "sends each entry and drops the marker after the current values",
			validateFunc: func() {
				updates := make(chan jetstream.KeyValueEntry, 3)
				first := mocks.NewMockKeyValueEntry(s.ctrl)
				second := mocks.NewMockKeyValueEntry(s.ctrl)
				updates <- first
				updates <- nil
				updates <- second
				close(updates)

				watcher := mocks.NewMockKeyWatcher(s.ctrl)
				watcher.EXPECT().Updates().Return(updates).AnyTimes()
				watcher.EXPECT().Stop().Return(nil)
				s.open()
				s.kv.EXPECT().Watch(gomock.Any(), ">", gomock.Any()).Return(watcher, nil)

				ch, err := s.client.KVWatch(s.ctx, "b", ">", jetstream.IncludeHistory())
				s.Require().NoError(err)

				var got []jetstream.KeyValueEntry
				for e := range ch {
					got = append(got, e)
				}
				s.Equal([]jetstream.KeyValueEntry{first, second}, got)
			},
		},
		{
			name: "when ctx ends stops the watch and closes the channel",
			validateFunc: func() {
				watcher := mocks.NewMockKeyWatcher(s.ctrl)
				watcher.EXPECT().Updates().Return(make(chan jetstream.KeyValueEntry)).AnyTimes()
				stopped := make(chan struct{})
				watcher.EXPECT().Stop().DoAndReturn(func() error {
					close(stopped)
					return nil
				})
				s.open()
				s.kv.EXPECT().Watch(gomock.Any(), ">").Return(watcher, nil)

				ctx, cancel := context.WithCancel(s.ctx)
				ch, err := s.client.KVWatch(ctx, "b", ">")
				s.Require().NoError(err)
				cancel()

				s.closes(ch)
				<-stopped
			},
		},
		{
			name: "when ctx ends while an entry waits closes the channel",
			validateFunc: func() {
				updates := make(chan jetstream.KeyValueEntry, 1)
				updates <- mocks.NewMockKeyValueEntry(s.ctrl)
				watcher := mocks.NewMockKeyWatcher(s.ctrl)
				watcher.EXPECT().Updates().Return(updates).AnyTimes()
				watcher.EXPECT().Stop().Return(nil)
				s.open()
				s.kv.EXPECT().Watch(gomock.Any(), ">").Return(watcher, nil)

				ctx, cancel := context.WithCancel(s.ctx)
				ch, err := s.client.KVWatch(ctx, "b", ">")
				s.Require().NoError(err)
				// Nobody reads the entry, so once the watch took it from
				// updates, its send waits until ctx ends.
				s.Eventually(
					func() bool { return len(updates) == 0 },
					time.Second,
					time.Millisecond,
				)
				cancel()

				s.closes(ch)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

// closes waits for ch to close, and fails when it does not.
func (s *KVPublicTestSuite) closes(
	ch <-chan jetstream.KeyValueEntry,
) {
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-timeout:
			s.Fail("the channel did not close")
			return
		}
	}
}

func TestKVPublicTestSuite(t *testing.T) {
	suite.Run(t, new(KVPublicTestSuite))
}

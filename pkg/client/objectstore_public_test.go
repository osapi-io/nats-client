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
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type ObjectStorePublicTestSuite struct {
	suite.Suite

	ctrl   *gomock.Controller
	js     *mocks.MockJetStream
	store  *mocks.MockObjectStore
	client *client.Client
	ctx    context.Context
}

func (s *ObjectStorePublicTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.js = mocks.NewMockJetStream(s.ctrl)
	s.store = mocks.NewMockObjectStore(s.ctrl)
	s.client = client.New(slog.Default(), &client.Options{})
	client.SetJetStream(s.client, s.js)
	s.ctx = context.Background()
}

func (s *ObjectStorePublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

// open makes the server return the handle of the bucket "b" once.
func (s *ObjectStorePublicTestSuite) open() {
	s.js.EXPECT().ObjectStore(gomock.Any(), "b").Return(s.store, nil)
}

func (s *ObjectStorePublicTestSuite) TestObjectStoreCreateOrUpdate() {
	cfg := jetstream.ObjectStoreConfig{Bucket: "b"}

	tests := []struct {
		name    string
		setup   func()
		wantErr string
	}{
		{
			name: "when the bucket is made keeps its handle",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateObjectStore(gomock.Any(), cfg).Return(s.store, nil)
			},
		},
		{
			name: "when only the storage type differs keeps the bucket",
			setup: func() {
				s.js.EXPECT().
					CreateOrUpdateObjectStore(gomock.Any(), cfg).
					Return(nil, errStorageType)
				s.js.EXPECT().ObjectStore(gomock.Any(), "b").Return(s.store, nil)
			},
		},
		{
			name: "when the bucket that is kept cannot be read returns the error",
			setup: func() {
				s.js.EXPECT().
					CreateOrUpdateObjectStore(gomock.Any(), cfg).
					Return(nil, errStorageType)
				s.js.EXPECT().ObjectStore(gomock.Any(), "b").Return(nil, errBoom)
			},
			wantErr: "failed to create/update Object Store bucket b: boom",
		},
		{
			name: "when the server refuses returns the error",
			setup: func() {
				s.js.EXPECT().CreateOrUpdateObjectStore(gomock.Any(), cfg).Return(nil, errBoom)
			},
			wantErr: "failed to create/update Object Store bucket b: boom",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.setup()

			err := s.client.ObjectStoreCreateOrUpdate(s.ctx, cfg)
			if tc.wantErr != "" {
				s.EqualError(err, tc.wantErr)
				return
			}
			s.Require().NoError(err)

			// The next call uses the handle, and does not ask the server.
			s.store.EXPECT().Delete(gomock.Any(), "o").Return(nil)
			s.NoError(s.client.ObjectDelete(s.ctx, "b", "o"))
		})
	}
}

func (s *ObjectStorePublicTestSuite) TestObjectPut() {
	info := &jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: "o"}}
	meta := jetstream.ObjectMeta{Name: "o"}
	reader := bytes.NewReader([]byte("data"))

	setup := func(st *mocks.MockObjectStore, err error) {
		st.EXPECT().Put(gomock.Any(), meta, reader).Return(info, err)
	}
	call := func(c *client.Client) (any, error) { return c.ObjectPut(s.ctx, "b", meta, reader) }

	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the answer of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, nil)

				got, err := call(s.client)
				s.NoError(err)
				s.Equal(info, got)
			},
		},
		{
			name: "returns the error of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, errBoom)

				_, err := call(s.client)
				s.ErrorIs(err, errBoom)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *ObjectStorePublicTestSuite) TestObjectPutBytes() {
	info := &jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: "o"}}

	setup := func(st *mocks.MockObjectStore, err error) {
		st.EXPECT().PutBytes(gomock.Any(), "o", []byte("data")).Return(info, err)
	}
	call := func(c *client.Client) (any, error) {
		return c.ObjectPutBytes(s.ctx, "b", "o", []byte("data"))
	}

	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the answer of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, nil)

				got, err := call(s.client)
				s.NoError(err)
				s.Equal(info, got)
			},
		},
		{
			name: "returns the error of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, errBoom)

				_, err := call(s.client)
				s.ErrorIs(err, errBoom)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *ObjectStorePublicTestSuite) TestObjectGetBytes() {
	setup := func(st *mocks.MockObjectStore, err error) {
		st.EXPECT().GetBytes(gomock.Any(), "o").Return([]byte("data"), err)
	}
	call := func(c *client.Client) (any, error) { return c.ObjectGetBytes(s.ctx, "b", "o") }

	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the answer of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, nil)

				got, err := call(s.client)
				s.NoError(err)
				s.Equal([]byte("data"), got)
			},
		},
		{
			name: "returns the error of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, errBoom)

				_, err := call(s.client)
				s.ErrorIs(err, errBoom)
			},
		},
		{
			name: "when the bucket is not there returns the error",
			validateFunc: func() {
				s.js.EXPECT().
					ObjectStore(gomock.Any(), "b").
					Return(nil, jetstream.ErrBucketNotFound)

				_, err := s.client.ObjectGetBytes(s.ctx, "b", "o")
				s.ErrorIs(err, jetstream.ErrBucketNotFound)
				s.ErrorContains(err, "failed to get Object Store bucket b")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *ObjectStorePublicTestSuite) TestObjectInfo() {
	info := &jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: "o"}}

	setup := func(st *mocks.MockObjectStore, err error) {
		st.EXPECT().GetInfo(gomock.Any(), "o").Return(info, err)
	}
	call := func(c *client.Client) (any, error) { return c.ObjectInfo(s.ctx, "b", "o") }

	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the answer of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, nil)

				got, err := call(s.client)
				s.NoError(err)
				s.Equal(info, got)
			},
		},
		{
			name: "returns the error of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, errBoom)

				_, err := call(s.client)
				s.ErrorIs(err, errBoom)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *ObjectStorePublicTestSuite) TestObjectList() {
	info := &jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: "o"}}

	setup := func(st *mocks.MockObjectStore, err error) {
		st.EXPECT().List(gomock.Any()).Return([]*jetstream.ObjectInfo{info}, err)
	}
	call := func(c *client.Client) (any, error) { return c.ObjectList(s.ctx, "b") }

	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the answer of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, nil)

				got, err := call(s.client)
				s.NoError(err)
				s.Equal([]*jetstream.ObjectInfo{info}, got)
			},
		},
		{
			name: "returns the error of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, errBoom)

				_, err := call(s.client)
				s.ErrorIs(err, errBoom)
			},
		},
		{
			name: "of an empty bucket returns an empty list",
			validateFunc: func() {
				s.open()
				s.store.EXPECT().List(gomock.Any()).Return(nil, jetstream.ErrNoObjectsFound)

				got, err := s.client.ObjectList(s.ctx, "b")
				s.NoError(err)
				s.Equal([]*jetstream.ObjectInfo{}, got)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *ObjectStorePublicTestSuite) TestObjectDelete() {
	setup := func(st *mocks.MockObjectStore, err error) {
		st.EXPECT().Delete(gomock.Any(), "o").Return(err)
	}
	call := func(c *client.Client) (any, error) { return nil, c.ObjectDelete(s.ctx, "b", "o") }

	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the answer of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, nil)

				got, err := call(s.client)
				s.NoError(err)
				s.Equal(nil, got)
			},
		},
		{
			name: "returns the error of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, errBoom)

				_, err := call(s.client)
				s.ErrorIs(err, errBoom)
			},
		},
		{
			name: "asks the server for the handle once",
			validateFunc: func() {
				s.open()
				s.store.EXPECT().Delete(gomock.Any(), "o").Return(nil).Times(2)

				s.NoError(s.client.ObjectDelete(s.ctx, "b", "o"))
				s.NoError(s.client.ObjectDelete(s.ctx, "b", "o"))
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *ObjectStorePublicTestSuite) TestObjectStoreStatus() {
	setup := func(st *mocks.MockObjectStore, err error) {
		st.EXPECT().Status(gomock.Any()).Return(nil, err)
	}
	call := func(c *client.Client) (any, error) {
		st, err := c.ObjectStoreStatus(s.ctx, "b")
		if st == nil {
			return nil, err
		}

		return st, err
	}

	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "returns the answer of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, nil)

				got, err := call(s.client)
				s.NoError(err)
				s.Equal(nil, got)
			},
		},
		{
			name: "returns the error of the bucket",
			validateFunc: func() {
				s.open()
				setup(s.store, errBoom)

				_, err := call(s.client)
				s.ErrorIs(err, errBoom)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func TestObjectStorePublicTestSuite(t *testing.T) {
	suite.Run(t, new(ObjectStorePublicTestSuite))
}

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
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type ObjectStorePublicTestSuite struct {
	suite.Suite

	mockCtrl    *gomock.Controller
	mockExt     *mocks.MockJetStream
	mockObjStor *mocks.MockObjectStore
	client      *client.Client
}

func (s *ObjectStorePublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockExt = mocks.NewMockJetStream(s.mockCtrl)
	s.mockObjStor = mocks.NewMockObjectStore(s.mockCtrl)
	s.client = client.New(slog.Default(), &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	})
	s.client.ExtJS = s.mockExt
}

func (s *ObjectStorePublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *ObjectStorePublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ObjectStorePublicTestSuite) TestCreateOrUpdateObjectStore() {
	tests := []struct {
		name        string
		config      jetstream.ObjectStoreConfig
		mockSetup   func()
		expectedErr string
	}{
		{
			name: "successfully creates Object Store bucket",
			config: jetstream.ObjectStoreConfig{
				Bucket: "file-uploads",
			},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateObjectStore(
						gomock.Any(),
						jetstream.ObjectStoreConfig{Bucket: "file-uploads"},
					).
					Return(s.mockObjStor, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name: "successfully creates Object Store bucket with custom config",
			config: jetstream.ObjectStoreConfig{
				Bucket:      "file-uploads",
				Description: "Storage for file uploads",
				TTL:         1 * time.Hour,
				MaxBytes:    500 * 1024 * 1024,
				Storage:     jetstream.FileStorage,
				Replicas:    1,
			},
			mockSetup: func() {
				expectedConfig := jetstream.ObjectStoreConfig{
					Bucket:      "file-uploads",
					Description: "Storage for file uploads",
					TTL:         1 * time.Hour,
					MaxBytes:    500 * 1024 * 1024,
					Storage:     jetstream.FileStorage,
					Replicas:    1,
				}
				s.mockExt.EXPECT().
					CreateOrUpdateObjectStore(gomock.Any(), expectedConfig).
					Return(s.mockObjStor, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name: "when storage type conflict returns existing bucket",
			config: jetstream.ObjectStoreConfig{
				Bucket:  "existing-bucket",
				Storage: jetstream.MemoryStorage,
			},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateObjectStore(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("stream configuration update can not change storage type")).
					Times(1)
				s.mockExt.EXPECT().
					ObjectStore(gomock.Any(), "existing-bucket").
					Return(s.mockObjStor, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name: "when storage type conflict and get fails returns original error",
			config: jetstream.ObjectStoreConfig{
				Bucket:  "bad-bucket",
				Storage: jetstream.MemoryStorage,
			},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateObjectStore(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("stream configuration update can not change storage type")).
					Times(1)
				s.mockExt.EXPECT().
					ObjectStore(gomock.Any(), "bad-bucket").
					Return(nil, errors.New("bucket not found")).
					Times(1)
			},
			expectedErr: "failed to create/update Object Store bucket bad-bucket: stream configuration update can not change storage type",
		},
		{
			name: "error creating Object Store bucket",
			config: jetstream.ObjectStoreConfig{
				Bucket: "bad-bucket",
			},
			mockSetup: func() {
				s.mockExt.EXPECT().
					CreateOrUpdateObjectStore(
						gomock.Any(),
						jetstream.ObjectStoreConfig{Bucket: "bad-bucket"},
					).
					Return(nil, errors.New("object store creation failed")).
					Times(1)
			},
			expectedErr: "failed to create/update Object Store bucket bad-bucket: object store creation failed",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			os, err := s.client.CreateOrUpdateObjectStore(context.Background(), tc.config)

			if tc.expectedErr == "" {
				s.NoError(err)
				s.NotNil(os)
			} else {
				s.EqualError(err, tc.expectedErr)
				s.Nil(os)
			}
		})
	}
}

func (s *ObjectStorePublicTestSuite) TestObjectStore() {
	tests := []struct {
		name        string
		bucketName  string
		mockSetup   func()
		expectedErr string
	}{
		{
			name:       "successfully gets Object Store bucket",
			bucketName: "file-uploads",
			mockSetup: func() {
				s.mockExt.EXPECT().
					ObjectStore(gomock.Any(), "file-uploads").
					Return(s.mockObjStor, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:       "error getting non-existent Object Store bucket",
			bucketName: "missing-bucket",
			mockSetup: func() {
				s.mockExt.EXPECT().
					ObjectStore(gomock.Any(), "missing-bucket").
					Return(nil, errors.New("object store not found")).
					Times(1)
			},
			expectedErr: "failed to get Object Store bucket missing-bucket: object store not found",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()

			os, err := s.client.ObjectStore(context.Background(), tc.bucketName)

			if tc.expectedErr == "" {
				s.NoError(err)
				s.NotNil(os)
			} else {
				s.EqualError(err, tc.expectedErr)
				s.Nil(os)
			}
		})
	}
}

func TestObjectStorePublicTestSuite(t *testing.T) {
	suite.Run(t, new(ObjectStorePublicTestSuite))
}

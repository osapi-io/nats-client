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
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type ConnectPublicTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockNATS *mocks.MockNATSConnector
	mockJS   *mocks.MockJetStreamContext
	client   *client.Client
}

func (s *ConnectPublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockNATS = mocks.NewMockNATSConnector(s.mockCtrl)
	s.mockJS = new(mocks.MockJetStreamContext)
	s.client = &client.Client{
		Opts: &client.Options{
			Host: "localhost",
			Port: 4222,
			Auth: client.AuthOptions{AuthType: client.NoAuth},
			Name: "test-client",
		},
		NC: s.mockNATS,
	}
}

func (s *ConnectPublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ConnectPublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *ConnectPublicTestSuite) TestConnect() {
	tests := []struct {
		name        string
		authType    client.AuthType
		mockSetup   func()
		expectedErr string
		needsNilNC  bool
	}{
		{
			name:     "successfully connects (NoAuth)",
			authType: client.NoAuth,
			mockSetup: func() {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)
				s.mockNATS.EXPECT().
					JetStream(gomock.Any()).
					Return(s.mockJS, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:     "successfully connects (UserPassAuth)",
			authType: client.UserPassAuth,
			mockSetup: func() {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)
				s.mockNATS.EXPECT().
					JetStream(gomock.Any()).
					Return(s.mockJS, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:     "successfully connects (NKeyAuth)",
			authType: client.NKeyAuth,
			mockSetup: func() {
				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				validSeed := []byte("SUAJT6TKTZNOL3IR2G6FTLZOKM2YSJVD7BL4TUSZCAMHISXNN2DHHXTS4Q")
				err := os.WriteFile(tempFile, validSeed, 0o644)
				require.NoError(s.T(), err)

				s.client.Opts.Auth.NKeyFile = tempFile

				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)
				s.mockNATS.EXPECT().
					JetStream(gomock.Any()).
					Return(s.mockJS, nil).
					Times(1)
			},
			expectedErr: "",
		},
		{
			name:     "fails to read NKey file",
			authType: client.NKeyAuth,
			mockSetup: func() {
				s.client.Opts.Auth.NKeyFile = "/invalid/path"
				s.mockNATS.EXPECT().
					JetStream().
					Times(0)
			},
			expectedErr: "failed to read nkey seed file",
		},
		{
			name:     "fails to parse NKey seed",
			authType: client.NKeyAuth,
			mockSetup: func() {
				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				err := os.WriteFile(tempFile, []byte("invalid-seed"), 0o644)
				require.NoError(s.T(), err)

				s.client.Opts.Auth.NKeyFile = tempFile
				s.mockNATS.EXPECT().
					JetStream().
					Times(0)
			},
			expectedErr: "failed to parse nkey seed",
		},
		{
			name:        "unsupported authentication method",
			authType:    client.AuthType(999),
			mockSetup:   func() {},
			expectedErr: "unsupported authentication method",
		},
		{
			name:     "fails to get public key from nkey",
			authType: client.NKeyAuth,
			mockSetup: func() {
				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				invalidSeed := []byte("INVALIDSEEDDATA")
				err := os.WriteFile(tempFile, invalidSeed, 0o644)
				require.NoError(s.T(), err)

				s.client.Opts.Auth.NKeyFile = tempFile

				s.mockNATS.EXPECT().
					JetStream().
					Times(0)
			},
			expectedErr: "failed to parse nkey seed",
		},
		{
			name:     "fails to get public key from nkey",
			authType: client.NKeyAuth,
			mockSetup: func() {
				mockKP := mocks.NewMockKeyPair(s.mockCtrl)
				mockKP.EXPECT().
					PublicKey().
					Return("", errors.New("simulated public key failure")).
					Times(1)

				s.client.KeyPair = mockKP

				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				validSeed := []byte("SUAJT6TKTZNOL3IR2G6FTLZOKM2YSJVD7BL4TUSZCAMHISXNN2DHHXTS4Q")
				err := os.WriteFile(tempFile, validSeed, 0o644)
				require.NoError(s.T(), err)

				s.client.Opts.Auth.NKeyFile = tempFile
			},
			expectedErr: "failed to get public key from nkey: simulated public key failure",
		},
		{
			name:     "error connecting to NATS",
			authType: client.NoAuth,
			mockSetup: func() {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("nats: connection error")).
					Times(1)
			},
			expectedErr: "error connecting to nats: nats: connection error",
		},
		// {
		// 	name: "error enabling External JetStream",
		// 	mockSetup: func() {
		// 		s.mockNATS.EXPECT().
		// 			Connect(gomock.Any(), gomock.Any()).
		// 			Return(&nats.Conn{}, nil).
		// 			Times(1)
		// 	},
		// 	overrideJS:  true,
		// 	expectedErr: "simulated JetStream error",
		// },
		{
			name:     "error enabling Native JetStream",
			authType: client.NoAuth,
			mockSetup: func() {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)

				s.mockNATS.EXPECT().
					JetStream(gomock.Any()).
					Return(nil, errors.New("error enabling native jetstream")).
					Times(1)
			},
			expectedErr: "error enabling native jetstream",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.client.Opts.Auth.AuthType = tc.authType
			tc.mockSetup()

			err := s.client.Connect()

			if tc.expectedErr == "" {
				s.NoError(err)
			} else {
				s.Contains(err.Error(), tc.expectedErr)
			}
		})
	}
}

func (s *ConnectPublicTestSuite) TestGetJetStreamCalledInConnect() {
	originalGetJetStream := client.GetJetStream
	defer func() { client.GetJetStream = originalGetJetStream }()

	client.GetJetStream = func(nc *nats.Conn) (jetstream.JetStream, error) {
		return nil, errors.New("simulated JetStream error")
	}

	s.mockNATS.EXPECT().
		Connect(gomock.Any(), gomock.Any()).
		Return(&nats.Conn{}, nil).
		Times(1)

	err := s.client.Connect()

	s.Error(err)
	s.Contains(err.Error(), "simulated JetStream error")
}

func TestConnectPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectPublicTestSuite))
}

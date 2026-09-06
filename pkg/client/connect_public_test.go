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
	"log/slog"
	"os"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type ConnectPublicTestSuite struct {
	suite.Suite

	mockCtrl *gomock.Controller
	mockNATS *mocks.MockNATSConnector
	client   *client.Client
}

func (s *ConnectPublicTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockNATS = mocks.NewMockNATSConnector(s.mockCtrl)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	opts := &client.Options{
		Host: "localhost",
		Port: 4222,
		Auth: client.AuthOptions{AuthType: client.NoAuth},
		Name: "test-client",
	}
	s.client = client.New(logger, opts)
	s.client.NC = s.mockNATS
}

func (s *ConnectPublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ConnectPublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *ConnectPublicTestSuite) TestConnect() {
	tests := []struct {
		name         string
		authType     client.AuthType
		mockSetup    func()
		validateFunc func(error)
	}{
		{
			name:     "successfully connects (NoAuth)",
			authType: client.NoAuth,
			mockSetup: func() {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)
				originalGetJetStream := client.GetJetStream
				client.GetJetStream = func(_ *nats.Conn) (jetstream.JetStream, error) {
					return mocks.NewMockJetStream(s.mockCtrl), nil
				}
				s.T().Cleanup(func() { client.GetJetStream = originalGetJetStream })
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:     "successfully connects (UserPassAuth)",
			authType: client.UserPassAuth,
			mockSetup: func() {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)
				originalGetJetStream := client.GetJetStream
				client.GetJetStream = func(_ *nats.Conn) (jetstream.JetStream, error) {
					return mocks.NewMockJetStream(s.mockCtrl), nil
				}
				s.T().Cleanup(func() { client.GetJetStream = originalGetJetStream })
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
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
				originalGetJetStream := client.GetJetStream
				client.GetJetStream = func(_ *nats.Conn) (jetstream.JetStream, error) {
					return mocks.NewMockJetStream(s.mockCtrl), nil
				}
				s.T().Cleanup(func() { client.GetJetStream = originalGetJetStream })
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:     "NKeyAuth signing callback invokes KeyPair Sign",
			authType: client.NKeyAuth,
			mockSetup: func() {
				mockKP := mocks.NewMockKeyPair(s.mockCtrl)
				mockKP.EXPECT().
					PublicKey().
					Return("test-pub-key", nil).
					Times(1)
				mockKP.EXPECT().
					Sign([]byte("test-nonce")).
					Return([]byte("signed-data"), nil).
					Times(1)

				s.client.KeyPair = mockKP

				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				validSeed := []byte("SUAJT6TKTZNOL3IR2G6FTLZOKM2YSJVD7BL4TUSZCAMHISXNN2DHHXTS4Q")
				err := os.WriteFile(tempFile, validSeed, 0o644)
				require.NoError(s.T(), err)

				s.client.Opts.Auth.NKeyFile = tempFile

				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ string, opts ...nats.Option) (*nats.Conn, error) {
						natsOpts := &nats.Options{}
						for _, opt := range opts {
							err := opt(natsOpts)
							require.NoError(s.T(), err)
						}
						sig, sigErr := natsOpts.SignatureCB([]byte("test-nonce"))
						require.NoError(s.T(), sigErr)
						require.Equal(s.T(), []byte("signed-data"), sig)
						return &nats.Conn{}, nil
					}).
					Times(1)
				originalGetJetStream := client.GetJetStream
				client.GetJetStream = func(_ *nats.Conn) (jetstream.JetStream, error) {
					return mocks.NewMockJetStream(s.mockCtrl), nil
				}
				s.T().Cleanup(func() { client.GetJetStream = originalGetJetStream })
			},
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:     "fails to read NKey file",
			authType: client.NKeyAuth,
			mockSetup: func() {
				s.client.Opts.Auth.NKeyFile = "/invalid/path"
			},
			validateFunc: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "failed to read nkey seed file")
			},
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
			},
			validateFunc: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "failed to parse nkey seed")
			},
		},
		{
			name:      "unsupported authentication method",
			authType:  client.AuthType(999),
			mockSetup: func() {},
			validateFunc: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "unsupported authentication method")
			},
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
			},
			validateFunc: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "failed to parse nkey seed")
			},
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
			validateFunc: func(err error) {
				s.Error(err)
				s.Contains(
					err.Error(),
					"failed to get public key from nkey: simulated public key failure",
				)
			},
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
			validateFunc: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "error connecting to nats: nats: connection error")
			},
		},
		{
			name:     "error enabling JetStream",
			authType: client.NoAuth,
			mockSetup: func() {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)

				// Override GetJetStream to simulate an error
				originalGetJetStream := client.GetJetStream
				client.GetJetStream = func(_ *nats.Conn) (jetstream.JetStream, error) {
					return nil, errors.New("error enabling jetstream")
				}
				s.T().Cleanup(func() { client.GetJetStream = originalGetJetStream })
			},
			validateFunc: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "error enabling jetstream")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.client.Opts.Auth.AuthType = tc.authType
			tc.mockSetup()

			tc.validateFunc(s.client.Connect())
		})
	}
}

func (s *ConnectPublicTestSuite) TestGetJetStream() {
	tests := []struct {
		name         string
		mockSetup    func()
		callFunc     func() (jetstream.JetStream, error)
		validateFunc func(jetstream.JetStream, error)
	}{
		{
			name:      "returns JetStream with default implementation",
			mockSetup: func() {},
			callFunc: func() (jetstream.JetStream, error) {
				return client.GetJetStream(&nats.Conn{})
			},
			validateFunc: func(js jetstream.JetStream, err error) {
				s.NoError(err)
				s.NotNil(js)
			},
		},
		{
			name: "error propagates when called in Connect",
			mockSetup: func() {
				originalGetJetStream := client.GetJetStream
				client.GetJetStream = func(_ *nats.Conn) (jetstream.JetStream, error) {
					return nil, errors.New("simulated JetStream error")
				}
				s.T().Cleanup(func() { client.GetJetStream = originalGetJetStream })

				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)
			},
			callFunc: func() (jetstream.JetStream, error) {
				return nil, s.client.Connect()
			},
			validateFunc: func(_ jetstream.JetStream, err error) {
				s.Error(err)
				s.Contains(err.Error(), "simulated JetStream error")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()
			tc.validateFunc(tc.callFunc())
		})
	}
}

func TestConnectPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectPublicTestSuite))
}

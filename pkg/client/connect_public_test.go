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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
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

	// release ends a dial of the mock that waits on it.
	release chan struct{}
	// closedConn closes when the mock connection is closed.
	closedConn chan struct{}
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
	s.release = make(chan struct{})
	s.closedConn = make(chan struct{})
}

func (s *ConnectPublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ConnectPublicTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *ConnectPublicTestSuite) TestConnect() {
	// capture makes the mock connector record the URL and the options of
	// the dial in got, and gives Connect a mock JetStream API.
	capture := func(_ context.CancelFunc, got *nats.Options) {
		s.mockNATS.EXPECT().
			Connect(gomock.Any(), gomock.Any()).
			DoAndReturn(func(u string, natsOpts ...nats.Option) (*nats.Conn, error) {
				got.Url = u
				for _, o := range natsOpts {
					s.Require().NoError(o(got))
				}

				return &nats.Conn{}, nil
			})
		s.stubJetStream()
	}
	// logs is what the client of the handler rows logs.
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	// reconnectSrv and reconnectLogs are the server and the logs of the
	// reconnect row.
	var reconnectSrv *server.Server
	var reconnectLogs syncBuffer

	tests := []struct {
		name     string
		authType client.AuthType
		ctx      func() (context.Context, context.CancelFunc)
		// opts, when set, makes a new client with these options and the
		// mock connector, in place of the suite's client.
		opts   *client.Options
		logger *slog.Logger
		// client, when set, gives a client of a real NATS server.
		client       func() *client.Client
		mockSetup    func(cancel context.CancelFunc, got *nats.Options)
		validateFunc func(c *client.Client, err error, got nats.Options)
	}{
		{
			name:     "successfully connects (NoAuth)",
			authType: client.NoAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
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
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.NoError(err)
			},
		},
		{
			name:     "successfully connects (UserPassAuth)",
			authType: client.UserPassAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
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
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.NoError(err)
			},
		},
		{
			name:     "successfully connects (NKeyAuth)",
			authType: client.NKeyAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				validSeed := []byte("SUAJT6TKTZNOL3IR2G6FTLZOKM2YSJVD7BL4TUSZCAMHISXNN2DHHXTS4Q")
				err := os.WriteFile(tempFile, validSeed, 0o644)
				require.NoError(s.T(), err)

				client.OptionsOf(s.client).Auth.NKeyFile = tempFile

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
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.NoError(err)
			},
		},
		{
			name:     "an injected KeyPair needs no seed file",
			authType: client.NKeyAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				mockKP := mocks.NewMockKeyPair(s.mockCtrl)
				mockKP.EXPECT().PublicKey().Return("test-pub-key", nil)
				client.SetKeyPair(s.client, mockKP)
				client.OptionsOf(s.client).Auth.NKeyFile = filepath.Join(
					s.T().TempDir(),
					"no-such.nkey",
				)

				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil)
				originalGetJetStream := client.GetJetStream
				client.GetJetStream = func(_ *nats.Conn) (jetstream.JetStream, error) {
					return mocks.NewMockJetStream(s.mockCtrl), nil
				}
				s.T().Cleanup(func() { client.GetJetStream = originalGetJetStream })
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.NoError(err)
			},
		},
		{
			name:     "NKeyAuth signing callback invokes KeyPair Sign",
			authType: client.NKeyAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				mockKP := mocks.NewMockKeyPair(s.mockCtrl)
				mockKP.EXPECT().
					PublicKey().
					Return("test-pub-key", nil).
					Times(1)
				mockKP.EXPECT().
					Sign([]byte("test-nonce")).
					Return([]byte("signed-data"), nil).
					Times(1)

				client.SetKeyPair(s.client, mockKP)

				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				validSeed := []byte("SUAJT6TKTZNOL3IR2G6FTLZOKM2YSJVD7BL4TUSZCAMHISXNN2DHHXTS4Q")
				err := os.WriteFile(tempFile, validSeed, 0o644)
				require.NoError(s.T(), err)

				client.OptionsOf(s.client).Auth.NKeyFile = tempFile

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
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.NoError(err)
			},
		},
		{
			name:     "successfully connects (CredsAuth)",
			authType: client.CredsAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				creds := fmt.Sprintf("%s/user.creds", s.T().TempDir())
				require.NoError(s.T(), os.WriteFile(creds, []byte("creds"), 0o600))
				client.OptionsOf(s.client).Auth.CredsFile = creds

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
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.NoError(err)
			},
		},
		{
			name:     "fails to read creds file",
			authType: client.CredsAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				client.OptionsOf(s.client).Auth.CredsFile = "/invalid/path"
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.Error(err)
				s.Contains(err.Error(), "failed to read creds file")
			},
		},
		{
			name:     "fails to read NKey file",
			authType: client.NKeyAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				client.OptionsOf(s.client).Auth.NKeyFile = "/invalid/path"
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.Error(err)
				s.Contains(err.Error(), "failed to read nkey seed file")
			},
		},
		{
			name:     "fails to parse NKey seed",
			authType: client.NKeyAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				err := os.WriteFile(tempFile, []byte("invalid-seed"), 0o644)
				require.NoError(s.T(), err)

				client.OptionsOf(s.client).Auth.NKeyFile = tempFile
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.Error(err)
				s.Contains(err.Error(), "failed to parse nkey seed")
			},
		},
		{
			name:      "unsupported authentication method",
			authType:  client.AuthType(999),
			mockSetup: func(context.CancelFunc, *nats.Options) {},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.Error(err)
				s.Contains(err.Error(), "unsupported authentication method")
			},
		},
		{
			name:     "fails to get public key from nkey",
			authType: client.NKeyAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				invalidSeed := []byte("INVALIDSEEDDATA")
				err := os.WriteFile(tempFile, invalidSeed, 0o644)
				require.NoError(s.T(), err)

				client.OptionsOf(s.client).Auth.NKeyFile = tempFile
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.Error(err)
				s.Contains(err.Error(), "failed to parse nkey seed")
			},
		},
		{
			name:     "fails to get public key from nkey",
			authType: client.NKeyAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				mockKP := mocks.NewMockKeyPair(s.mockCtrl)
				mockKP.EXPECT().
					PublicKey().
					Return("", errors.New("simulated public key failure")).
					Times(1)

				client.SetKeyPair(s.client, mockKP)

				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				validSeed := []byte("SUAJT6TKTZNOL3IR2G6FTLZOKM2YSJVD7BL4TUSZCAMHISXNN2DHHXTS4Q")
				err := os.WriteFile(tempFile, validSeed, 0o644)
				require.NoError(s.T(), err)

				client.OptionsOf(s.client).Auth.NKeyFile = tempFile
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
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
			mockSetup: func(context.CancelFunc, *nats.Options) {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("nats: connection error")).
					Times(1)
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.Error(err)
				s.Contains(err.Error(), "error connecting to nats: nats: connection error")
			},
		},
		{
			name:     "error enabling JetStream",
			authType: client.NoAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)
				// The connection that JetStream fails on is closed, not leaked.
				s.mockNATS.EXPECT().Close().Times(1)

				// Override GetJetStream to simulate an error
				originalGetJetStream := client.GetJetStream
				client.GetJetStream = func(_ *nats.Conn) (jetstream.JetStream, error) {
					return nil, errors.New("error enabling jetstream")
				}
				s.T().Cleanup(func() { client.GetJetStream = originalGetJetStream })
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.Error(err)
				s.Contains(err.Error(), "error enabling jetstream")
			},
		},
		{
			name:     "with nil options and a nil logger returns ErrNoOptions",
			authType: client.NoAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				s.client = client.New(nil, nil)
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.ErrorIs(err, client.ErrNoOptions)
			},
		},
		{
			name:     "a second Connect without Close returns ErrConnected",
			authType: client.NoAuth,
			mockSetup: func(context.CancelFunc, *nats.Options) {
				client.SetJetStream(s.client, mocks.NewMockJetStream(s.mockCtrl))
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.ErrorIs(err, client.ErrConnected)
			},
		},
		{
			name: "a ctx that already ended dials nothing",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx, cancel
			},
			mockSetup: func(context.CancelFunc, *nats.Options) {},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.ErrorIs(err, context.Canceled)
				s.Nil(client.JetStreamOf(s.client))
			},
		},
		{
			name: "the deadline of ctx is the dial timeout",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 300*time.Millisecond)
			},
			mockSetup: func(_ context.CancelFunc, got *nats.Options) {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ string, natsOpts ...nats.Option) (*nats.Conn, error) {
						for _, o := range natsOpts {
							s.Require().NoError(o(got))
						}

						return &nats.Conn{}, nil
					})
				s.stubJetStream()
			},
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.NoError(err)
				s.Positive(got.Timeout)
				s.LessOrEqual(got.Timeout, 300*time.Millisecond)
			},
		},
		{
			name: "a ctx that ends while the dial runs closes the connection",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			mockSetup: func(cancel context.CancelFunc, _ *nats.Options) {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					DoAndReturn(func(string, ...nats.Option) (*nats.Conn, error) {
						cancel()

						return &nats.Conn{}, nil
					})
				s.expectClose()
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.ErrorIs(err, context.Canceled)
				s.Nil(client.JetStreamOf(s.client))
				<-s.closedConn
			},
		},
		{
			name: "a ctx that ends returns at once, and the late connection is closed",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			mockSetup: func(cancel context.CancelFunc, _ *nats.Options) {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					DoAndReturn(func(string, ...nats.Option) (*nats.Conn, error) {
						cancel()
						<-s.release

						return &nats.Conn{}, nil
					})
				s.expectClose()
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				// Connect returned while the dial still waits.
				s.ErrorIs(err, context.Canceled)
				s.Nil(client.JetStreamOf(s.client))
				s.ErrorIs(s.client.Connect(context.Background()), client.ErrConnected)

				close(s.release)
				<-s.closedConn
			},
		},
		{
			name: "Close while the dial runs ends Connect, and closes the late connection",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			mockSetup: func(context.CancelFunc, *nats.Options) {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					DoAndReturn(func(string, ...nats.Option) (*nats.Conn, error) {
						// Close does not wait for the lock of the dial.
						s.client.Close()
						<-s.release

						return &nats.Conn{}, nil
					})
				s.expectClose()
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.ErrorIs(err, context.Canceled)
				s.Nil(client.JetStreamOf(s.client))

				close(s.release)
				<-s.closedConn
			},
		},
		{
			name: "a Close after the dial returned, before Connect ends, closes the connection",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			mockSetup: func(context.CancelFunc, *nats.Options) {
				s.T().Cleanup(client.SetAfterDial(s.client.Close))
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil)
				s.expectClose()
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.ErrorIs(err, context.Canceled)
				s.Nil(client.JetStreamOf(s.client))
				<-s.closedConn
			},
		},
		{
			name: "a failed dial after ctx ended closes nothing",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			mockSetup: func(cancel context.CancelFunc, _ *nats.Options) {
				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					DoAndReturn(func(string, ...nats.Option) (*nats.Conn, error) {
						cancel()
						<-s.release
						defer close(s.closedConn)

						return nil, errors.New("no servers available")
					})
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.ErrorIs(err, context.Canceled)

				close(s.release)
				<-s.closedConn
			},
		},
		{
			name: "an error of GetJetStream closes the connection and is returned",
			mockSetup: func(context.CancelFunc, *nats.Options) {
				originalGetJetStream := client.GetJetStream
				client.GetJetStream = func(_ *nats.Conn) (jetstream.JetStream, error) {
					return nil, errors.New("simulated JetStream error")
				}
				s.T().Cleanup(func() { client.GetJetStream = originalGetJetStream })

				s.mockNATS.EXPECT().
					Connect(gomock.Any(), gomock.Any()).
					Return(&nats.Conn{}, nil).
					Times(1)
				s.mockNATS.EXPECT().Close().Times(1)
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.Error(err)
				s.Contains(err.Error(), "simulated JetStream error")
			},
		},
		{
			name:      "the zero value of Reconnect tries forever, DefaultReconnectWait apart",
			opts:      &client.Options{Host: "localhost", Port: 4222},
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				s.Equal(-1, got.MaxReconnect)
				s.Equal(client.DefaultReconnectWait, got.ReconnectWait)
				s.False(got.RetryOnFailedConnect)
			},
		},
		{
			name: "the Reconnect options set the tries, the wait and the first connect",
			opts: &client.Options{
				Host: "localhost",
				Port: 4222,
				Reconnect: client.ReconnectOptions{
					MaxAttempts:       3,
					Wait:              time.Second,
					RetryFirstConnect: true,
				},
			},
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				s.Equal(3, got.MaxReconnect)
				s.Equal(time.Second, got.ReconnectWait)
				s.True(got.RetryOnFailedConnect)
			},
		},
		{
			name:      "dials a host name at nats://<host>:<port>",
			opts:      &client.Options{Host: "nats.example", Port: 4222},
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				s.Equal("nats://nats.example:4222", got.Url)
			},
		},
		{
			name:      "dials an IPv6 address in brackets",
			opts:      &client.Options{Host: "::1", Port: 4222},
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				s.Equal("nats://[::1]:4222", got.Url)
			},
		},
		{
			name:      "the handlers log: a drop that the client asked for is info",
			opts:      &client.Options{Host: "localhost", Port: 4222},
			logger:    logger,
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				logs.Reset()
				got.DisconnectedErrCB(nil, nil)
				s.Contains(logs.String(), `level=INFO msg="disconnected from NATS"`)
			},
		},
		{
			name:      "the handlers log: a drop with an error is a warning with the error",
			opts:      &client.Options{Host: "localhost", Port: 4222},
			logger:    logger,
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				logs.Reset()
				got.DisconnectedErrCB(nil, errors.New("eof"))
				s.Contains(logs.String(), `level=WARN msg="disconnected from NATS" error=eof`)
			},
		},
		{
			name:      "the handlers log: a reconnect is info with the URL",
			opts:      &client.Options{Host: "localhost", Port: 4222},
			logger:    logger,
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				logs.Reset()
				got.ReconnectedCB(nil)
				s.Contains(logs.String(), `level=INFO msg="reconnected to NATS" url=""`)
			},
		},
		{
			name:      "the handlers log: a close with the last error is an error",
			opts:      &client.Options{Host: "localhost", Port: 4222},
			logger:    logger,
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				logs.Reset()
				got.ClosedCB(nil)
				s.Contains(
					logs.String(),
					`level=ERROR msg="NATS connection closed" error="nats: invalid connection"`,
				)
			},
		},
		{
			name:      "the handlers log: a close with no error is info",
			opts:      &client.Options{Host: "localhost", Port: 4222},
			logger:    logger,
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				logs.Reset()
				got.ClosedCB(&nats.Conn{})
				s.Contains(logs.String(), `level=INFO msg="NATS connection closed"`)
			},
		},
		{
			name:      "the handlers log: an error of the server names the subject",
			opts:      &client.Options{Host: "localhost", Port: 4222},
			logger:    logger,
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				logs.Reset()
				got.AsyncErrorCB(nil, &nats.Subscription{Subject: "jobs.x"}, nats.ErrSlowConsumer)
				s.Contains(
					logs.String(),
					`level=ERROR msg="NATS error" error="nats: slow consumer, messages dropped" subject=jobs.x`,
				)
			},
		},
		{
			name:      "the handlers log: an error with no subscription has no subject",
			opts:      &client.Options{Host: "localhost", Port: 4222},
			logger:    logger,
			mockSetup: capture,
			validateFunc: func(_ *client.Client, err error, got nats.Options) {
				s.Require().NoError(err)
				logs.Reset()
				got.AsyncErrorCB(nil, nil, errors.New("permissions violation"))
				s.Contains(
					logs.String(),
					`level=ERROR msg="NATS error" error="permissions violation"`,
				)
			},
		},
		{
			name: "on a TLS server, a client that trusts its certificate connects with TLS",
			client: func() *client.Client {
				port, roots := startTLSServer(s.T())

				return client.New(slog.New(slog.DiscardHandler), &client.Options{
					Host: "127.0.0.1",
					Port: port,
					TLS:  &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
				})
			},
			validateFunc: func(c *client.Client, err error, _ nats.Options) {
				s.Require().NoError(err)
				s.Contains(c.ConnectedURL(), "tls://")
				c.Close()
			},
		},
		{
			name: "on a TLS server, a client that does not trust its certificate is refused",
			client: func() *client.Client {
				port, _ := startTLSServer(s.T())

				return client.New(slog.New(slog.DiscardHandler), &client.Options{
					Host: "127.0.0.1",
					Port: port,
					TLS:  &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12},
				})
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.ErrorContains(err, "certificate")
			},
		},
		{
			name: "on a TLS server, a client with no TLS is refused",
			client: func() *client.Client {
				port, _ := startTLSServer(s.T())

				return client.New(slog.New(slog.DiscardHandler), &client.Options{
					Host: "127.0.0.1",
					Port: port,
				})
			},
			validateFunc: func(_ *client.Client, err error, _ nats.Options) {
				s.Error(err)
			},
		},
		{
			name: "on a server, a call while the client closes and connects is its result or ErrNotConnected",
			client: func() *client.Client {
				srv := startServer(
					s.T(),
					&server.Options{
						Host:      "127.0.0.1",
						Port:      -1,
						JetStream: true,
						StoreDir:  s.T().TempDir(),
					},
				)
				c := client.New(slog.New(slog.DiscardHandler), &client.Options{
					Host: "127.0.0.1",
					Port: srv.Addr().(*net.TCPAddr).Port,
				})
				s.T().Cleanup(c.Close)

				return c
			},
			validateFunc: func(c *client.Client, err error, _ nats.Options) {
				s.Require().NoError(err)
				ctx := context.Background()
				stop := make(chan struct{})
				var calls sync.WaitGroup
				var mu sync.Mutex
				var unexpected []error

				calls.Add(1)
				go func() {
					defer calls.Done()
					for {
						select {
						case <-stop:
							return
						default:
						}
						_ = c.ConnectedURL()
						_ = c.ConnectedServerVersion()
						for _, err := range []error{
							c.PublishCore(ctx, "x", []byte("y")),
							subscribe(ctx, c),
						} {
							if err != nil && !isClosedOrNotConnected(err) {
								mu.Lock()
								unexpected = append(unexpected, err)
								mu.Unlock()
							}
						}
					}
				}()

				for range 20 {
					c.Close()
					s.Require().NoError(c.Connect(context.Background()))
				}
				close(stop)
				calls.Wait()

				s.Empty(unexpected)
				s.NotEmpty(c.ConnectedURL())
			},
		},
		{
			name: "on a server, logs the drop and the reconnect when the server comes back",
			client: func() *client.Client {
				reconnectSrv = startServer(s.T(), &server.Options{Host: "127.0.0.1", Port: -1})
				c := client.New(slog.New(slog.NewTextHandler(&reconnectLogs, nil)), &client.Options{
					Host:      "127.0.0.1",
					Port:      reconnectSrv.Addr().(*net.TCPAddr).Port,
					Reconnect: client.ReconnectOptions{Wait: 50 * time.Millisecond},
				})
				s.T().Cleanup(c.Close)

				return c
			},
			validateFunc: func(c *client.Client, err error, _ nats.Options) {
				s.Require().NoError(err)
				port := reconnectSrv.Addr().(*net.TCPAddr).Port
				reconnectSrv.Shutdown()
				reconnectSrv.WaitForShutdown()
				s.Eventually(func() bool {
					return strings.Contains(reconnectLogs.String(), `msg="disconnected from NATS"`)
				}, 5*time.Second, 10*time.Millisecond)

				startServer(s.T(), &server.Options{Host: "127.0.0.1", Port: port})
				s.Eventually(func() bool {
					return strings.Contains(reconnectLogs.String(), `msg="reconnected to NATS"`)
				}, 5*time.Second, 10*time.Millisecond)
				s.NotEmpty(c.ConnectedURL())
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			client.OptionsOf(s.client).Auth.AuthType = tc.authType
			ctx, cancel := context.WithCancel(context.Background())
			if tc.ctx != nil {
				ctx, cancel = tc.ctx()
			}
			defer cancel()
			got := nats.GetDefaultOptions()
			if tc.mockSetup != nil {
				tc.mockSetup(cancel, &got)
			}

			c := s.client
			switch {
			case tc.client != nil:
				c = tc.client()
			case tc.opts != nil:
				l := tc.logger
				if l == nil {
					l = slog.Default()
				}
				c = client.New(l, tc.opts)
				c.NC = s.mockNATS
			}

			tc.validateFunc(c, c.Connect(ctx), got)
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
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.mockSetup()
			tc.validateFunc(tc.callFunc())
		})
	}
}

// connectWith connects a client of opts and logger, and returns the URL and the
// nats.Options that Connect gave to NATS.

// expectClose expects one Close of the mock connection, which closes
// s.closedConn.
func (s *ConnectPublicTestSuite) expectClose() {
	s.mockNATS.EXPECT().
		Close().
		Do(func() { close(s.closedConn) }).
		Times(1)
}

// stubJetStream makes Connect get a mock JetStream API.
func (s *ConnectPublicTestSuite) stubJetStream() {
	orig := client.GetJetStream
	client.GetJetStream = func(*nats.Conn) (jetstream.JetStream, error) {
		return mocks.NewMockJetStream(s.mockCtrl), nil
	}
	s.T().Cleanup(func() { client.GetJetStream = orig })
}

func TestConnectPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectPublicTestSuite))
}

// startTLSServer starts a NATS server with JetStream and TLS on a free
// port, with a certificate that it makes. It returns the port and a pool
// that trusts the certificate. The cleanup of t stops the server.
func startTLSServer(
	t *testing.T,
) (int, *x509.CertPool) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(cert)

	srv, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
			MinVersion:   tls.VersionTLS12,
		},
	})
	require.NoError(t, err)
	srv.Start()
	require.True(t, srv.ReadyForConnections(5*time.Second))
	t.Cleanup(srv.Shutdown)

	return srv.Addr().(*net.TCPAddr).Port, roots
}

// ReconnectServerPublicTestSuite stops a real NATS server under a connected
// client and starts it again, because only a real connection shows the
// reconnect and its log.

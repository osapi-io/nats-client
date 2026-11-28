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
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:     "an injected KeyPair needs no seed file",
			authType: client.NKeyAuth,
			mockSetup: func() {
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
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:     "successfully connects (CredsAuth)",
			authType: client.CredsAuth,
			mockSetup: func() {
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
			validateFunc: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:     "fails to read creds file",
			authType: client.CredsAuth,
			mockSetup: func() {
				client.OptionsOf(s.client).Auth.CredsFile = "/invalid/path"
			},
			validateFunc: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "failed to read creds file")
			},
		},
		{
			name:     "fails to read NKey file",
			authType: client.NKeyAuth,
			mockSetup: func() {
				client.OptionsOf(s.client).Auth.NKeyFile = "/invalid/path"
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

				client.OptionsOf(s.client).Auth.NKeyFile = tempFile
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

				client.OptionsOf(s.client).Auth.NKeyFile = tempFile
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

				client.SetKeyPair(s.client, mockKP)

				tempDir := s.T().TempDir()
				tempFile := fmt.Sprintf("%s/test.nkey", tempDir)

				validSeed := []byte("SUAJT6TKTZNOL3IR2G6FTLZOKM2YSJVD7BL4TUSZCAMHISXNN2DHHXTS4Q")
				err := os.WriteFile(tempFile, validSeed, 0o644)
				require.NoError(s.T(), err)

				client.OptionsOf(s.client).Auth.NKeyFile = tempFile
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
				// The connection that JetStream fails on is closed, not leaked.
				s.mockNATS.EXPECT().Close().Times(1)

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
		{
			name:     "with nil options and a nil logger returns ErrNoOptions",
			authType: client.NoAuth,
			mockSetup: func() {
				s.client = client.New(nil, nil)
			},
			validateFunc: func(err error) {
				s.ErrorIs(err, client.ErrNoOptions)
			},
		},
		{
			name:     "a second Connect without Close returns ErrConnected",
			authType: client.NoAuth,
			mockSetup: func() {
				client.SetJetStream(s.client, mocks.NewMockJetStream(s.mockCtrl))
			},
			validateFunc: func(err error) {
				s.ErrorIs(err, client.ErrConnected)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			client.OptionsOf(s.client).Auth.AuthType = tc.authType
			tc.mockSetup()

			tc.validateFunc(s.client.Connect(context.Background()))
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
				s.mockNATS.EXPECT().Close().Times(1)
			},
			callFunc: func() (jetstream.JetStream, error) {
				return nil, s.client.Connect(context.Background())
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

// connectWith connects a client of opts and logger, and returns the URL and the
// nats.Options that Connect gave to NATS.
func (s *ConnectPublicTestSuite) connectWith(
	logger *slog.Logger,
	opts *client.Options,
) (string, nats.Options) {
	c := client.New(logger, opts)
	c.NC = s.mockNATS
	var url string
	got := nats.GetDefaultOptions()
	s.mockNATS.EXPECT().
		Connect(gomock.Any(), gomock.Any()).
		DoAndReturn(func(u string, natsOpts ...nats.Option) (*nats.Conn, error) {
			url = u
			for _, o := range natsOpts {
				s.Require().NoError(o(&got))
			}
			return &nats.Conn{}, nil
		})
	orig := client.GetJetStream
	client.GetJetStream = func(*nats.Conn) (jetstream.JetStream, error) {
		return mocks.NewMockJetStream(s.mockCtrl), nil
	}
	s.T().Cleanup(func() { client.GetJetStream = orig })
	s.Require().NoError(c.Connect(context.Background()))

	return url, got
}

func (s *ConnectPublicTestSuite) TestReconnectPolicy() {
	tests := []struct {
		name      string
		reconnect client.ReconnectOptions
		wantMax   int
		wantWait  time.Duration
		wantRetry bool
	}{
		{
			name:     "the zero value tries forever, DefaultReconnectWait apart",
			wantMax:  -1,
			wantWait: client.DefaultReconnectWait,
		},
		{
			name: "the options set the tries, the wait and the first connect",
			reconnect: client.ReconnectOptions{
				MaxAttempts:       3,
				Wait:              time.Second,
				RetryFirstConnect: true,
			},
			wantMax:   3,
			wantWait:  time.Second,
			wantRetry: true,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			_, got := s.connectWith(slog.Default(), &client.Options{
				Host:      "localhost",
				Port:      4222,
				Reconnect: tc.reconnect,
			})
			s.Equal(tc.wantMax, got.MaxReconnect)
			s.Equal(tc.wantWait, got.ReconnectWait)
			s.Equal(tc.wantRetry, got.RetryOnFailedConnect)
		})
	}
}

func (s *ConnectPublicTestSuite) TestConnectionHandlers() {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	_, got := s.connectWith(logger, &client.Options{Host: "localhost", Port: 4222})

	tests := []struct {
		name string
		call func()
		want string
	}{
		{
			name: "a drop that the client asked for is info",
			call: func() { got.DisconnectedErrCB(nil, nil) },
			want: `level=INFO msg="disconnected from NATS"`,
		},
		{
			name: "a drop with an error is a warning with the error",
			call: func() { got.DisconnectedErrCB(nil, errors.New("eof")) },
			want: `level=WARN msg="disconnected from NATS" error=eof`,
		},
		{
			name: "a reconnect is info with the URL",
			call: func() { got.ReconnectedCB(nil) },
			want: `level=INFO msg="reconnected to NATS" url=""`,
		},
		{
			name: "a close with the last error is an error",
			call: func() { got.ClosedCB(nil) },
			want: `level=ERROR msg="NATS connection closed" error="nats: invalid connection"`,
		},
		{
			name: "a close with no error is info",
			call: func() { got.ClosedCB(&nats.Conn{}) },
			want: `level=INFO msg="NATS connection closed"`,
		},
		{
			name: "an error of the server names the subject",
			call: func() {
				got.AsyncErrorCB(nil, &nats.Subscription{Subject: "jobs.x"}, nats.ErrSlowConsumer)
			},
			want: `level=ERROR msg="NATS error" error="nats: slow consumer, messages dropped" subject=jobs.x`,
		},
		{
			name: "an error with no subscription has no subject",
			call: func() { got.AsyncErrorCB(nil, nil, errors.New("permissions violation")) },
			want: `level=ERROR msg="NATS error" error="permissions violation"`,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			buf.Reset()
			tc.call()
			s.Contains(buf.String(), tc.want)
		})
	}
}

func (s *ConnectPublicTestSuite) TestConnectContext() {
	tests := []struct {
		name         string
		ctx          func() (context.Context, context.CancelFunc)
		mockSetup    func(cancel context.CancelFunc, got *nats.Options)
		validateFunc func(err error, got nats.Options)
	}{
		{
			name: "a ctx that already ended dials nothing",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx, cancel
			},
			mockSetup: func(context.CancelFunc, *nats.Options) {},
			validateFunc: func(err error, _ nats.Options) {
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
			validateFunc: func(err error, got nats.Options) {
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
				s.mockNATS.EXPECT().Close().Times(1)
			},
			validateFunc: func(err error, _ nats.Options) {
				s.ErrorIs(err, context.Canceled)
				s.Nil(client.JetStreamOf(s.client))
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			ctx, cancel := tc.ctx()
			defer cancel()
			got := nats.GetDefaultOptions()
			tc.mockSetup(cancel, &got)

			tc.validateFunc(s.client.Connect(ctx), got)
		})
	}
}

// stubJetStream makes Connect get a mock JetStream API.
func (s *ConnectPublicTestSuite) stubJetStream() {
	orig := client.GetJetStream
	client.GetJetStream = func(*nats.Conn) (jetstream.JetStream, error) {
		return mocks.NewMockJetStream(s.mockCtrl), nil
	}
	s.T().Cleanup(func() { client.GetJetStream = orig })
}

func (s *ConnectPublicTestSuite) TestConnectURL() {
	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "a name", host: "nats.example", want: "nats://nats.example:4222"},
		{name: "an IPv6 address is in brackets", host: "::1", want: "nats://[::1]:4222"},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			url, _ := s.connectWith(slog.Default(), &client.Options{Host: tc.host, Port: 4222})
			s.Equal(tc.want, url)
		})
	}
}

func TestConnectPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectPublicTestSuite))
}

// ConnectTLSPublicTestSuite connects to a real NATS server that serves TLS,
// because only a real handshake shows that the client checks the server.
type ConnectTLSPublicTestSuite struct {
	suite.Suite

	server *server.Server
	port   int
	roots  *x509.CertPool
}

func (s *ConnectTLSPublicTestSuite) SetupTest() {
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
	s.Require().NoError(err)
	cert, err := x509.ParseCertificate(der)
	s.Require().NoError(err)
	s.roots = x509.NewCertPool()
	s.roots.AddCert(cert)

	srv, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  s.T().TempDir(),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
			MinVersion:   tls.VersionTLS12,
		},
	})
	s.Require().NoError(err)
	srv.Start()
	s.Require().True(srv.ReadyForConnections(5 * time.Second))
	s.server = srv
	s.port = srv.Addr().(*net.TCPAddr).Port
}

func (s *ConnectTLSPublicTestSuite) TearDownTest() {
	s.server.Shutdown()
}

func (s *ConnectTLSPublicTestSuite) TestConnect() {
	tests := []struct {
		name         string
		tls          *tls.Config
		validateFunc func(*client.Client, error)
	}{
		{
			name: "when the client trusts the server's certificate, it connects with TLS",
			tls:  &tls.Config{RootCAs: s.roots, MinVersion: tls.VersionTLS12},
			validateFunc: func(c *client.Client, err error) {
				s.Require().NoError(err)
				s.Contains(c.ConnectedURL(), "tls://")
				c.Close()
			},
		},
		{
			name: "when the client does not trust the certificate, it is refused",
			tls:  &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12},
			validateFunc: func(_ *client.Client, err error) {
				s.ErrorContains(err, "certificate")
			},
		},
		{
			name: "when the client has no TLS, the server refuses it",
			validateFunc: func(_ *client.Client, err error) {
				s.Error(err)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			c := client.New(slog.New(slog.DiscardHandler), &client.Options{
				Host: "127.0.0.1",
				Port: s.port,
				TLS:  tc.tls,
			})
			tc.validateFunc(c, c.Connect(context.Background()))
		})
	}
}

func TestConnectTLSPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectTLSPublicTestSuite))
}

// ReconnectServerPublicTestSuite stops a real NATS server under a connected
// client and starts it again, because only a real connection shows the
// reconnect and its log.

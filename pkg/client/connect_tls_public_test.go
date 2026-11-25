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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
)

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
			tc.validateFunc(c, c.Connect())
		})
	}
}

func TestConnectTLSPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectTLSPublicTestSuite))
}

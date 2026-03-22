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

package client

import (
	"log/slog"
	"os"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
)

type ConnectionTestSuite struct {
	suite.Suite
}

func (s *ConnectionTestSuite) TestConnectedServerVersion() {
	tests := []struct {
		name         string
		setupFunc    func() *Client
		validateFunc func(string)
	}{
		{
			name: "returns version from real NATSConnWrapper with non-nil Conn",
			setupFunc: func() *Client {
				logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
				c := New(logger, &Options{Host: "localhost", Port: 4222})
				// Use a real NATSConnWrapper with a non-nil Conn.
				// ConnectedServerVersion returns "" when not actually
				// connected, but this exercises the code path.
				c.NC = &NATSConnWrapper{Conn: &nats.Conn{}}

				return c
			},
			validateFunc: func(version string) {
				// Not connected, so version is empty, but the
				// code path through wrapper.Conn is exercised.
				s.Empty(version)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			c := tc.setupFunc()
			tc.validateFunc(c.ConnectedServerVersion())
		})
	}
}

func TestConnectionTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionTestSuite))
}

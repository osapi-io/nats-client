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

package client

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConnectTestSuite struct {
	suite.Suite

	client *Client
	logger *slog.Logger
}

func (s *ConnectTestSuite) SetupTest() {
	s.logger = slog.Default()

	s.client = &Client{
		logger: s.logger,
		Opts: &Options{
			Host: "localhost",
			Port: 4222,
			Auth: AuthOptions{
				AuthType: NoAuth,
			},
		},
	}
}

func (s *ConnectTestSuite) TestGetAuthTypeName() {
	tests := []struct {
		name     string
		authType AuthType
		expected string
	}{
		{
			name:     "no auth returns none",
			authType: NoAuth,
			expected: "none",
		},
		{
			name:     "user pass auth returns user_pass",
			authType: UserPassAuth,
			expected: "user_pass",
		},
		{
			name:     "nkey auth returns nkey",
			authType: NKeyAuth,
			expected: "nkey",
		},
		{
			name:     "unknown auth type returns unknown",
			authType: AuthType(99),
			expected: "unknown",
		},
		{
			name:     "negative auth type returns unknown",
			authType: AuthType(-1),
			expected: "unknown",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.client.Opts.Auth.AuthType = tc.authType
			result := s.client.getAuthTypeName()
			s.Equal(tc.expected, result)
		})
	}
}

func TestConnectTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectTestSuite))
}

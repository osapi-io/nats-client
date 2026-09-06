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
	"log/slog"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/osapi-io/nats-client/pkg/client"
)

type ClientPublicTestSuite struct {
	suite.Suite
}

func (s *ClientPublicTestSuite) TestNew() {
	tests := []struct {
		name         string
		opts         *client.Options
		validateFunc func(*client.Client)
	}{
		{
			name: "keeps the options it was given",
			opts: &client.Options{
				Host: "localhost",
				Port: 4222,
				Name: "test-client",
				Auth: client.AuthOptions{AuthType: client.NoAuth},
			},
			validateFunc: func(c *client.Client) {
				s.NotNil(c)
				s.Equal("localhost", c.Opts.Host)
				s.Equal(4222, c.Opts.Port)
				s.Equal("test-client", c.Opts.Name)
				s.Equal(client.NoAuth, c.Opts.Auth.AuthType)
			},
		},
		{
			name: "starts with a connector so Connect has one to call",
			opts: &client.Options{
				Host: "localhost",
				Port: 4222,
			},
			validateFunc: func(c *client.Client) {
				s.NotNil(c.NC)
				s.IsType(&client.NATSConnWrapper{}, c.NC)
			},
		},
		{
			name: "leaves JetStream unset until Connect runs",
			opts: &client.Options{
				Host: "localhost",
				Port: 4222,
			},
			validateFunc: func(c *client.Client) {
				s.Nil(c.ExtJS)
				s.Nil(c.KeyPair)
			},
		},
		{
			name: "accepts nil options",
			opts: nil,
			validateFunc: func(c *client.Client) {
				s.NotNil(c)
				s.Nil(c.Opts)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.validateFunc(client.New(slog.Default(), tc.opts))
		})
	}
}

func TestClientPublicTestSuite(t *testing.T) {
	suite.Run(t, new(ClientPublicTestSuite))
}

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
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

// GetJetStream is a public variable function wrapping jetstream.New.
var GetJetStream = func(nc *nats.Conn) (jetstream.JetStream, error) {
	return jetstream.New(nc)
}

// Connect establishes the connection to the NATS server and JetStream context.
// This method returns an error if there are any issues during connection.
func (c *Client) Connect() error {
	natsURL := fmt.Sprintf("nats://%s:%d", c.Opts.Host, c.Opts.Port)

	c.logger.Debug(
		"connecting to NATS server",
		slog.String("url", natsURL),
		slog.String("auth_type", c.getAuthTypeName()),
		slog.String("client_name", c.Opts.Name),
	)

	opts, err := c.connectOptions()
	if err != nil {
		return err
	}

	nc, err := c.NC.Connect(natsURL, opts...)
	if err != nil {
		return fmt.Errorf("error connecting to nats: %w", err)
	}

	extJS, err := GetJetStream(nc)
	if err != nil {
		return fmt.Errorf("error enabling jetstream: %w", err)
	}
	c.ExtJS = extJS

	c.logger.Debug("successfully connected to NATS and enabled JetStream")

	return nil
}

// connectOptions builds the dial options for the configured authentication
// method.
//
// Separated from Connect so that adding an auth type is a case here rather
// than another branch in a function that also dials, enables JetStream and
// logs.
func (c *Client) connectOptions() ([]nats.Option, error) {
	var opts []nats.Option

	if c.Opts.Name != "" {
		opts = append(opts, nats.Name(c.Opts.Name))
	}

	switch c.Opts.Auth.AuthType {
	case NoAuth:
		return opts, nil

	case UserPassAuth:
		return append(
			opts,
			nats.UserInfo(c.Opts.Auth.Username, c.Opts.Auth.Password),
		), nil

	case NKeyAuth:
		opt, err := c.nkeyOption()
		if err != nil {
			return nil, err
		}

		return append(opts, opt), nil

	default:
		return nil, errors.New("unsupported authentication method")
	}
}

// nkeyOption reads the configured seed file and returns the signing option it
// implies.
//
// c.KeyPair, when set, replaces the seed file — the tests inject a keypair
// rather than writing a real seed to disk.
func (c *Client) nkeyOption() (nats.Option, error) {
	seed, err := os.ReadFile(c.Opts.Auth.NKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read nkey seed file: %w", err)
	}

	kp := c.KeyPair
	if kp == nil {
		kp, err = nkeys.FromSeed(seed)
		if err != nil {
			return nil, fmt.Errorf("failed to parse nkey seed: %w", err)
		}
	}

	pubKey, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get public key from nkey: %w", err)
	}

	return nats.Nkey(pubKey, func(nonce []byte) ([]byte, error) {
		return kp.Sign(nonce)
	}), nil
}

// getAuthTypeName returns a human-readable string for the auth type
func (c *Client) getAuthTypeName() string {
	switch c.Opts.Auth.AuthType {
	case NoAuth:
		return "none"
	case UserPassAuth:
		return "user_pass"
	case NKeyAuth:
		return "nkey"
	default:
		return "unknown"
	}
}

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
	"fmt"
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
	var nc *nats.Conn
	var err error

	var opts []nats.Option
	if c.Opts.Name != "" {
		opts = append(opts, nats.Name(c.Opts.Name))
	}

	switch c.Opts.Auth.AuthType {
	case NoAuth:
		nc, err = c.NC.Connect(natsURL, opts...)
	case UserPassAuth:
		opts = append(opts, nats.UserInfo(c.Opts.Auth.Username, c.Opts.Auth.Password))
		nc, err = c.NC.Connect(natsURL, opts...)
	case NKeyAuth:
		seed, readErr := os.ReadFile(c.Opts.Auth.NKeyFile)
		if readErr != nil {
			return fmt.Errorf("failed to read nkey seed file: %w", readErr)
		}

		kp, kpErr := nkeys.FromSeed(seed)
		if kpErr != nil {
			return fmt.Errorf("failed to parse nkey seed: %w", kpErr)
		}

		pubKey, pubErr := kp.PublicKey()
		if pubErr != nil {
			return fmt.Errorf("failed to get public key from nkey: %w", pubErr)
		}

		// This code isn't covered in unit tests because the signing function inside
		// nats.Nkey() is only executed when the client attempts to authenticate with
		// a live NATS server. Since unit tests don't spin up a real server, this
		// callback is never triggered during testing.
		opts = append(opts, nats.Nkey(pubKey, func(nonce []byte) ([]byte, error) {
			return kp.Sign(nonce)
		}))

		nc, err = c.NC.Connect(natsURL, opts...)
	default:
		return fmt.Errorf("unsupported authentication method")
	}

	if err != nil {
		return fmt.Errorf("error connecting to nats: %w", err)
	}

	extJS, err := GetJetStream(nc)
	if err != nil {
		return fmt.Errorf("error enabling jetstream: %w", err)
	}
	c.ExtJS = extJS

	nativeJS, err := c.NC.JetStream()
	if err != nil {
		return fmt.Errorf("error enabling native jetstream: %w", err)
	}
	c.NativeJS = nativeJS

	return nil
}

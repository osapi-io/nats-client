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
	"net"
	"os"
	"strconv"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

// ErrConnected is the error of a Connect on a client that is connected.
var ErrConnected = errors.New("already connected: call Close first")

// GetJetStream is a public variable function wrapping jetstream.New.
var GetJetStream = func(nc *nats.Conn) (jetstream.JetStream, error) {
	return jetstream.New(nc)
}

// Connect establishes the connection to the NATS server and JetStream context.
// This method returns an error if there are any issues during connection.
//
// A second Connect without a Close between returns ErrConnected, so that the
// first connection does not leak.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ExtJS != nil {
		return ErrConnected
	}

	scheme := "nats"
	if c.Opts.TLS != nil {
		scheme = "tls"
	}
	natsURL := scheme + "://" + net.JoinHostPort(c.Opts.Host, strconv.Itoa(c.Opts.Port))

	c.logger.Debug(
		"connecting to NATS server",
		slog.String("url", natsURL),
		slog.String("auth_type", c.getAuthTypeName()),
		slog.String("client_name", c.Opts.Name),
	)

	var nc *nats.Conn
	var err error

	opts := c.reconnectOptions()
	if c.Opts.Name != "" {
		opts = append(opts, nats.Name(c.Opts.Name))
	}
	if c.Opts.TLS != nil {
		opts = append(opts, nats.Secure(c.Opts.TLS))
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

		var kp nkeys.KeyPair
		if c.KeyPair != nil {
			kp = c.KeyPair // mocked
		} else {
			var kpErr error
			kp, kpErr = nkeys.FromSeed(seed)
			if kpErr != nil {
				return fmt.Errorf("failed to parse nkey seed: %w", kpErr)
			}
		}

		pubKey, pubErr := kp.PublicKey()
		if pubErr != nil {
			return fmt.Errorf("failed to get public key from nkey: %w", pubErr)
		}

		opts = append(opts, nats.Nkey(pubKey, func(nonce []byte) ([]byte, error) {
			return kp.Sign(nonce)
		}))

		nc, err = c.NC.Connect(natsURL, opts...)
	case CredsAuth:
		// nats.UserCredentials reads the file on each connect, so a missing
		// file is reported here rather than as a failed handshake.
		if _, statErr := os.Stat(c.Opts.Auth.CredsFile); statErr != nil {
			return fmt.Errorf("failed to read creds file: %w", statErr)
		}
		opts = append(opts, nats.UserCredentials(c.Opts.Auth.CredsFile))
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
	// A call that raced the last Close can store a handle of the old
	// connection after Close forgot them.
	c.forgetHandles()
	c.ExtJS = extJS

	c.logger.Debug("successfully connected to NATS and enabled JetStream")

	return nil
}

// reconnectOptions returns the options of the reconnect policy of the
// client, with a handler that logs each change of the connection.
func (c *Client) reconnectOptions() []nats.Option {
	policy := c.Opts.Reconnect
	attempts := policy.MaxAttempts
	if attempts == 0 {
		attempts = -1
	}
	wait := policy.Wait
	if wait == 0 {
		wait = DefaultReconnectWait
	}

	return []nats.Option{
		nats.MaxReconnects(attempts),
		nats.ReconnectWait(wait),
		nats.RetryOnFailedConnect(policy.RetryFirstConnect),
		nats.DisconnectErrHandler(c.disconnected),
		nats.ReconnectHandler(c.reconnected),
		nats.ClosedHandler(c.closed),
		nats.ErrorHandler(c.asyncError),
	}
}

// disconnected logs a drop of the connection. err is nil for a drop that the
// client asked for.
func (c *Client) disconnected(
	_ *nats.Conn,
	err error,
) {
	if err == nil {
		c.logger.Info("disconnected from NATS")
		return
	}
	c.logger.Warn("disconnected from NATS", slog.String("error", err.Error()))
}

// reconnected logs a connection that is up again.
func (c *Client) reconnected(
	nc *nats.Conn,
) {
	c.logger.Info("reconnected to NATS", slog.String("url", nc.ConnectedUrl()))
}

// closed logs the end of the connection: after Close, or after the last try of
// a reconnect. Each call after it fails.
func (c *Client) closed(
	nc *nats.Conn,
) {
	if err := nc.LastError(); err != nil {
		c.logger.Error("NATS connection closed", slog.String("error", err.Error()))
		return
	}
	c.logger.Info("NATS connection closed")
}

// asyncError logs an error that the server sent outside a call, such as a
// permissions violation or a slow consumer.
func (c *Client) asyncError(
	_ *nats.Conn,
	sub *nats.Subscription,
	err error,
) {
	attrs := []any{slog.String("error", err.Error())}
	if sub != nil {
		attrs = append(attrs, slog.String("subject", sub.Subject))
	}
	c.logger.Error("NATS error", attrs...)
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
	case CredsAuth:
		return "creds"
	default:
		return "unknown"
	}
}

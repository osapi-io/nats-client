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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

// ErrConnected is the error of a Connect on a client that is connected.
var ErrConnected = errors.New("already connected: call Close first")

// ErrNoOptions is the error of a Connect on a client that New made with nil
// options.
var ErrNoOptions = errors.New("no options: give New the options of the connection")

// GetJetStream is a public variable function wrapping jetstream.New.
var GetJetStream = func(nc *nats.Conn) (jetstream.JetStream, error) {
	return jetstream.New(nc)
}

// Connect establishes the connection to the NATS server and JetStream context.
// This method returns an error if there are any issues during connection.
//
// ctx bounds the dial. Its deadline is the dial timeout of each server, in
// place of the default of nats.go. A ctx that ends before the dial, or while
// it runs, ends Connect at once with ctx's error, and leaves no connection: a
// connection that the dial makes after that is closed. A Close while the dial
// runs ends Connect in the same way.
//
// Connect does not hold the lock of the client while it dials, so Close and
// the other calls do not wait for the dial.
//
// A second Connect without a Close between, or while a first Connect dials,
// returns ErrConnected, so that the first connection does not leak.
func (c *Client) Connect(
	ctx context.Context,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("connect to nats: %w", err)
	}

	dialCtx, natsURL, opts, err := c.beginConnect(ctx)
	if err != nil {
		return err
	}

	r, late := c.dial(dialCtx, natsURL, opts)
	if late {
		return fmt.Errorf("connect to nats: %w", dialCtx.Err())
	}
	afterDial()

	return c.finishConnect(dialCtx, r)
}

// afterDial runs between a dial that returned in time and the end of
// Connect. Tests set it, to run a Close in that gap.
var afterDial = func() {}

// beginConnect checks the state and the options under the lock, marks the
// client as connecting, and returns the context of the dial, the URL and the
// options of nats.go.
func (c *Client) beginConnect(
	ctx context.Context,
) (context.Context, string, []nats.Option, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.js != nil || c.cancelDial != nil {
		return nil, "", nil, ErrConnected
	}
	if c.opts == nil {
		return nil, "", nil, ErrNoOptions
	}

	scheme := "nats"
	if c.opts.TLS != nil {
		scheme = "tls"
	}
	natsURL := scheme + "://" + net.JoinHostPort(c.opts.Host, strconv.Itoa(c.opts.Port))

	c.logger.Debug(
		"connecting to NATS server",
		slog.String("url", natsURL),
		slog.String("auth_type", c.getAuthTypeName()),
		slog.String("client_name", c.opts.Name),
	)

	opts := c.reconnectOptions()
	if c.opts.Name != "" {
		opts = append(opts, nats.Name(c.opts.Name))
	}
	if c.opts.TLS != nil {
		opts = append(opts, nats.Secure(c.opts.TLS))
	}
	if deadline, ok := ctx.Deadline(); ok {
		opts = append(opts, nats.Timeout(max(time.Until(deadline), time.Millisecond)))
	}

	auth, err := c.authOptions()
	if err != nil {
		return nil, "", nil, err
	}
	opts = append(opts, auth...)

	dialCtx, cancel := context.WithCancel(ctx)
	c.cancelDial = cancel

	return dialCtx, natsURL, opts, nil
}

// authOptions returns the options of nats.go for the authentication of the
// options.
func (c *Client) authOptions() ([]nats.Option, error) {
	switch c.opts.Auth.AuthType {
	case NoAuth:
		return nil, nil
	case UserPassAuth:
		return []nats.Option{
			nats.UserInfo(c.opts.Auth.Username, c.opts.Auth.Password),
		}, nil
	case NKeyAuth:
		kp, err := c.nkeyPair()
		if err != nil {
			return nil, err
		}

		pubKey, err := kp.PublicKey()
		if err != nil {
			return nil, fmt.Errorf("failed to get public key from nkey: %w", err)
		}

		return []nats.Option{
			nats.Nkey(pubKey, func(nonce []byte) ([]byte, error) {
				return kp.Sign(nonce)
			}),
		}, nil
	case CredsAuth:
		// nats.UserCredentials reads the file on each connect, so a missing
		// file is reported here rather than as a failed handshake.
		if _, err := os.Stat(c.opts.Auth.CredsFile); err != nil {
			return nil, fmt.Errorf("failed to read creds file: %w", err)
		}

		return []nats.Option{nats.UserCredentials(c.opts.Auth.CredsFile)}, nil
	default:
		return nil, fmt.Errorf("unsupported authentication method")
	}
}

// dialResult is the connection, or the error, of one dial.
type dialResult struct {
	nc  *nats.Conn
	err error
}

// dial connects to natsURL in a goroutine, and waits until it ends or ctx
// ends. late is true when ctx ended first, and the result is then empty.
// Then the dial goes on, and the connection that it makes is closed. The
// client stays connecting until then, so that no Connect dials beside it.
func (c *Client) dial(
	ctx context.Context,
	natsURL string,
	opts []nats.Option,
) (dialResult, bool) {
	done := make(chan dialResult, 1)
	go func() {
		nc, err := c.NC.Connect(natsURL, opts...)
		done <- dialResult{nc: nc, err: err}
	}()

	select {
	case r := <-done:
		return r, false
	case <-ctx.Done():
		go func() {
			if r := <-done; r.err == nil {
				c.NC.Close()
			}

			c.mu.Lock()
			c.cancelDial()
			c.cancelDial = nil
			c.mu.Unlock()
		}()

		return dialResult{}, true
	}
}

// finishConnect ends a dial that returned in time. Under the lock, it stores
// the JetStream API of the connection of r, or closes the connection when
// ctx ended or JetStream could not start.
func (c *Client) finishConnect(
	ctx context.Context,
	r dialResult,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// A Close since the dial returned ended ctx. Read it before the cancel
	// below ends it too.
	ended := ctx.Err()
	c.cancelDial()
	c.cancelDial = nil

	if r.err != nil {
		return fmt.Errorf("error connecting to nats: %w", r.err)
	}
	if ended != nil {
		c.NC.Close()

		return fmt.Errorf("connect to nats: %w", ended)
	}

	extJS, err := GetJetStream(r.nc)
	if err != nil {
		// The connection would keep trying to reconnect, with no client to
		// close it.
		c.NC.Close()
		return fmt.Errorf("error enabling jetstream: %w", err)
	}
	// A call that raced the last Close can store a handle of the old
	// connection after Close forgot them.
	c.forgetHandles()
	c.js = extJS

	c.logger.Debug("successfully connected to NATS and enabled JetStream")

	return nil
}

// nkeyPair returns the key pair of the connection: the injected KeyPair, or
// the one in the seed file of the options. The seed bytes are cleared after
// they are parsed.
func (c *Client) nkeyPair() (nkeys.KeyPair, error) {
	if c.keyPair != nil {
		return c.keyPair, nil
	}
	seed, err := os.ReadFile(c.opts.Auth.NKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read nkey seed file: %w", err)
	}
	defer clear(seed)
	kp, err := nkeys.FromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse nkey seed: %w", err)
	}

	return kp, nil
}

// reconnectOptions returns the options of the reconnect policy of the
// client, with a handler that logs each change of the connection.
func (c *Client) reconnectOptions() []nats.Option {
	policy := c.opts.Reconnect
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
	switch c.opts.Auth.AuthType {
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

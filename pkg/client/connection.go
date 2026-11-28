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
	"errors"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// ErrNotConnected is the error of a call before Connect, or after Close.
var ErrNotConnected = errors.New("not connected: call Connect first")

// JetStream returns the JetStream API of the connection, for a call that this
// client does not wrap, or ErrNotConnected.
func (c *Client) JetStream() (jetstream.JetStream, error) {
	c.mu.RLock()
	js := c.js
	c.mu.RUnlock()
	if js == nil {
		return nil, ErrNotConnected
	}

	return js, nil
}

// withConn runs fn on the connection while Close and Connect wait, or returns
// ErrNotConnected.
func (c *Client) withConn(
	fn func(NATSConnector) error,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.js == nil || c.NC == nil {
		return ErrNotConnected
	}

	return fn(c.NC)
}

// remember stores handle as the handle of name in handles, only while js is
// still the JetStream of the connection. A call that took js before a Close
// and finished after the next Connect does not keep a handle of the closed
// connection.
func (c *Client) remember(
	handles *sync.Map,
	js jetstream.JetStream,
	name string,
	handle any,
) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.js == js {
		handles.Store(name, handle)
	}
}

// gone reports whether err says that a bucket or a stream is not on the
// server, or that the connection of its handle closed.
func gone(
	err error,
) bool {
	return errors.Is(err, nats.ErrConnectionClosed) ||
		errors.Is(err, jetstream.ErrBucketNotFound) ||
		errors.Is(err, jetstream.ErrStreamNotFound) ||
		errors.Is(err, jetstream.ErrNoStreamResponse)
}

// forget drops the handle of name from handles when err says that it is gone,
// so that the next call asks the server again, and returns err.
func forget(
	handles *sync.Map,
	name string,
	err error,
) error {
	if gone(err) {
		handles.Delete(name)
	}

	return err
}

// Close closes the underlying NATS connection, and forgets the handles of the
// buckets and streams. Connect can then connect again. A call that runs while
// the client closes returns its result or ErrNotConnected.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancelDial != nil {
		// A Connect dials. It closes the connection that the dial makes.
		c.cancelDial()

		return
	}
	if c.NC != nil {
		c.NC.Close()
	}
	c.js = nil
	c.forgetHandles()
}

// forgetHandles drops the handle of each bucket and stream, so that the next
// call asks the server again.
func (c *Client) forgetHandles() {
	c.kvs.Clear()
	c.objectStores.Clear()
	c.streams.Clear()
}

// ConnectedURL returns the URL of the NATS server the client is connected to.
// Returns an empty string if not connected.
func (c *Client) ConnectedURL() string {
	var url string
	_ = c.withConn(func(nc NATSConnector) error {
		url = nc.ConnectedUrl()
		return nil
	})

	return url
}

// ConnectedServerVersion returns the version of the connected NATS server.
// Returns an empty string if not connected or the version is unavailable.
func (c *Client) ConnectedServerVersion() string {
	var version string
	_ = c.withConn(func(nc NATSConnector) error {
		version = nc.ConnectedServerVersion()
		return nil
	})

	return version
}

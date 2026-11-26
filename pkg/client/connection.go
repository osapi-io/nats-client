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

	"github.com/nats-io/nats.go/jetstream"
)

// ErrNotConnected is the error of a call to JetStream before Connect, or
// after Close.
var ErrNotConnected = errors.New("not connected: call Connect first")

// jetStream returns the JetStream API of the connection, or ErrNotConnected.
func (c *Client) jetStream() (jetstream.JetStream, error) {
	if c.ExtJS == nil {
		return nil, ErrNotConnected
	}

	return c.ExtJS, nil
}

// gone reports whether err says that a bucket or a stream is not on the
// server.
func gone(
	err error,
) bool {
	return errors.Is(err, jetstream.ErrBucketNotFound) ||
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
// buckets and streams. Connect can then connect again.
func (c *Client) Close() {
	if c.NC != nil {
		c.NC.Close()
	}
	c.ExtJS = nil
	c.kvs.Clear()
	c.objectStores.Clear()
	c.streams.Clear()
}

// ConnectedURL returns the URL of the NATS server the client is connected to.
// Returns an empty string if not connected.
func (c *Client) ConnectedURL() string {
	if c.NC == nil {
		return ""
	}

	return c.NC.ConnectedUrl()
}

// ConnectedServerVersion returns the version of the connected NATS server.
// Returns an empty string if not connected or the version is unavailable.
func (c *Client) ConnectedServerVersion() string {
	if c.NC == nil {
		return ""
	}

	return c.NC.ConnectedServerVersion()
}

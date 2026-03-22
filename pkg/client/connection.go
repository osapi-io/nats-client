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
	"context"

	"github.com/nats-io/nats.go/jetstream"
)

// Close closes the underlying NATS connection.
func (c *Client) Close() {
	if c.NC != nil {
		c.NC.Close()
	}
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

	wrapper, ok := c.NC.(*NATSConnWrapper)
	if !ok || wrapper.Conn == nil {
		return ""
	}

	return wrapper.Conn.ConnectedServerVersion()
}

// KeyValue returns a handle to a JetStream Key-Value bucket by name.
func (c *Client) KeyValue(
	ctx context.Context,
	bucket string,
) (jetstream.KeyValue, error) {
	return c.ExtJS.KeyValue(ctx, bucket)
}

// Stream returns a handle to a JetStream stream by name.
func (c *Client) Stream(
	ctx context.Context,
	name string,
) (jetstream.Stream, error) {
	return c.ExtJS.Stream(ctx, name)
}

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
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

// SetJetStream sets the JetStream API of c, as Connect does, for a test that
// has no server.
func SetJetStream(
	c *Client,
	js jetstream.JetStream,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.js = js
}

// JetStreamOf is the JetStream API of c, or nil when c is not connected.
func JetStreamOf(
	c *Client,
) jetstream.JetStream {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.js
}

// SetKeyPair sets the nkey of NKeyAuth in place of the file.
func SetKeyPair(
	c *Client,
	kp nkeys.KeyPair,
) {
	c.keyPair = kp
}

// KeyPairOf is the nkey that c was given, or nil.
func KeyPairOf(
	c *Client,
) nkeys.KeyPair {
	return c.keyPair
}

// OptionsOf is the options that New gave c.
func OptionsOf(
	c *Client,
) *Options {
	return c.opts
}

// SetAfterDial sets what Connect runs between a dial that returned in time
// and its end, and returns a func that restores it.
func SetAfterDial(
	fn func(),
) func() {
	old := afterDial
	afterDial = fn

	return func() { afterDial = old }
}

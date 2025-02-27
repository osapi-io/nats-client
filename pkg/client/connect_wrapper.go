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

import "github.com/nats-io/nats.go"

// NATSConnector defines an interface for managing a NATS connection.
type NATSConnector interface {
	JetStream(opts ...nats.JSOpt) (nats.JetStreamContext, error)
	Close()
	ConnectedUrl() string
	Connect(url string, opts ...nats.Option) (*nats.Conn, error)
}

// NATSConnWrapper is a concrete implementation of NATSConnector, wrapping a *nats.Conn.
type NATSConnWrapper struct {
	Conn *nats.Conn
}

// JetStream wraps the JetStream method of nats.Conn.
func (n *NATSConnWrapper) JetStream(opts ...nats.JSOpt) (nats.JetStreamContext, error) {
	return n.Conn.JetStream(opts...)
}

// Close wraps the Close method of nats.Conn.
func (n *NATSConnWrapper) Close() {
	n.Conn.Close()
}

// ConnectedUrl wraps the ConnectedUrl method of nats.Conn.
func (n *NATSConnWrapper) ConnectedUrl() string {
	return n.Conn.ConnectedUrl()
}

func (n *NATSConnWrapper) Connect(url string, opts ...nats.Option) (*nats.Conn, error) {
	conn, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, err
	}
	n.Conn = conn
	return conn, nil
}

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
	"log/slog"

	"github.com/nats-io/nats.go"
)

// AuthType defines the different authentication methods supported by the client.
type AuthType int

const (
	// NoAuth represents a connection with no authentication.
	NoAuth AuthType = iota
	// UserPassAuth represents authentication using a username and password.
	UserPassAuth
	// NKeyAuth represents authentication using NATS NKEYs (Ed25519 public-private key pairs).
	NKeyAuth
)

// ClientOptions holds the configuration for connecting to a NATS server,
// including connection details and authentication settings.
type ClientOptions struct {
	// Host specifies the NATS server hostname or IP address.
	Host string
	// Port specifies the NATS server port.
	Port int
	// Auth contains authentication settings for connecting to the NATS server.
	Auth AuthOptions
}

// AuthOptions holds authentication-related settings for connecting to NATS.
type AuthOptions struct {
	// AuthType specifies the authentication method to use (NoAuth, UserPassAuth, or NKeyAuth).
	AuthType AuthType
	// Username is required for UserPassAuth and represents the NATS username.
	Username string
	// Password is required for UserPassAuth and represents the NATS password.
	Password string
	// NKeyFile is required for NKeyAuth and specifies the path to the NKEY private seed file.
	// This file should contain an Ed25519 private key (starting with "S").
	NKeyFile string
}

// Client provides an implementation for interacting with an embedded NATS server.
type Client struct {
	logger *slog.Logger
}

// StreamConfig extends nats.StreamConfig to include custom settings for an embedded NATS server stream configuration.
type StreamConfig struct {
	// StreamConfig embeds nats.StreamConfig, which defines the core stream settings
	// such as name, subjects, storage type, and replication factor.
	*nats.StreamConfig

	// Consumers represents the list of consumer configurations associated with this stream.
	// Each consumer defines how messages from the stream are consumed and acknowledged.
	Consumers []*ConsumerConfig
}

// ConsumerConfig extends nats.ConsumerConfig to include custom settings for an embedded NATS server consumer configuration.
type ConsumerConfig struct {
	// ConsumerConfig embeds nats.ConsumerConfig, which includes configurations
	// such as durable name, acknowledgment policy, max deliver attempts, and more.
	*nats.ConsumerConfig
}

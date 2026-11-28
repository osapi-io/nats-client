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
	"strings"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/propagation"
)

var _ propagation.TextMapCarrier = headerCarrier{}

// headerCarrier is a propagation.TextMapCarrier over the headers of a NATS
// message. NATS header keys are case sensitive, and a client in another
// language writes the W3C keys in lower case, such as "traceparent". Set
// keeps the key that the propagator gives, and Get finds a key in any case.
type headerCarrier nats.Header

// Get returns the first value of key, also when the message has the key in
// a different case.
func (h headerCarrier) Get(
	key string,
) string {
	if v := nats.Header(h).Get(key); v != "" {
		return v
	}
	for k, v := range h {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return v[0]
		}
	}

	return ""
}

// Set sets key to value, with key as it is.
func (h headerCarrier) Set(
	key string,
	value string,
) {
	h[key] = []string{value}
}

// Keys returns the keys of the headers.
func (h headerCarrier) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}

	return keys
}

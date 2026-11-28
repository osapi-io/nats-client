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
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
)

// CarrierTestSuite tests Keys, which the propagators that the exported
// API uses do not call.
type CarrierTestSuite struct {
	suite.Suite
}

func (s *CarrierTestSuite) TestKeys() {
	tests := []struct {
		name         string
		header       nats.Header
		validateFunc func([]string)
	}{
		{
			name:   "returns each key as the message has it",
			header: nats.Header{"traceparent": {"x"}, "Nats-Msg-Id": {"1"}},
			validateFunc: func(keys []string) {
				s.ElementsMatch([]string{"traceparent", "Nats-Msg-Id"}, keys)
			},
		},
		{
			name:   "returns no keys for no headers",
			header: nats.Header{},
			validateFunc: func(keys []string) {
				s.Empty(keys)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			tc.validateFunc(headerCarrier(tc.header).Keys())
		})
	}
}

func TestCarrierTestSuite(t *testing.T) {
	suite.Run(t, new(CarrierTestSuite))
}

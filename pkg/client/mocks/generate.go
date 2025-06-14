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

package mocks

//go:generate go tool github.com/golang/mock/mockgen -destination=./jetstream_native.gen.go -package=mocks github.com/nats-io/nats.go JetStreamContext
//go:generate go tool github.com/golang/mock/mockgen -destination=./jetstream_ext.gen.go -package=mocks github.com/nats-io/nats.go/jetstream JetStream,Consumer,MessageBatch,Msg
//go:generate go tool github.com/golang/mock/mockgen -destination=./kv.gen.go -package=mocks github.com/nats-io/nats.go KeyValue,KeyValueEntry,KeyWatcher

//go:generate go tool github.com/golang/mock/mockgen -destination=./nats_connector.gen.go -package=mocks github.com/osapi-io/nats-client/pkg/client NATSConnector
//go:generate go tool github.com/golang/mock/mockgen -destination=./nkeys.gen.go -package=mocks github.com/nats-io/nkeys KeyPair

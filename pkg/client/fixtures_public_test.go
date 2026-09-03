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
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, EXPRESS OR IMPLIED,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package client_test

import "time"

// Fixtures shared by the public tests in this package.
//
// They live in one file because several are used from more than one test file,
// and a value repeated across files is the duplication `add-constant` exists to
// catch — the same bucket named in three places drifts to three names.

// Timings. Three rather than one because some cases must out-wait a poll and
// others must be cut short before a reply arrives.
const (
	shortTimeout   = 50 * time.Millisecond
	defaultTimeout = 100 * time.Millisecond
	longTimeout    = 200 * time.Millisecond
)

// natsDefaultPort is the port a NATS server listens on unless told otherwise.
const natsDefaultPort = 4222

// testFileMode is the permission the nkey seed fixtures are written with.
const testFileMode = 0o644

// Names the tests address. One spelling each, so a rename is one edit.
const (
	testBucket    = "test-bucket"
	testKey       = "test-key"
	testValue     = "test-value"
	testStream    = "test-stream"
	testSubject   = "test.subject"
	testWildcard  = "test.*"
	notifySubject = "notify.subject"
	badBucket     = "bad-bucket"
)

// Sizes and counts the mocks report.
const (
	msgChanBuffer   = 10
	updateChanSize  = 3
	testMsgCount    = 10
	testByteCount   = 1024
	testRevision    = 42
	maxBucketBytes  = 100 * 1024 * 1024
	maxObjectBytes  = 500 * 1024 * 1024
	testMaxInFlight = 5
)

// unknownAuthType is deliberately outside the defined AuthType values, so the
// tests can exercise the default branch.
const unknownAuthType = 999

// testNkeyPath is the seed file layout the auth tests write and read back.
const testNkeyPath = "%s/test.nkey"

// Payloads and names the mocks return. Each appears in several tests, and a
// value repeated across files drifts to several spellings.
const (
	testEntryValue       = `{"status": "ok"}`
	testDataValue        = `{"test": "data"}`
	testConsumer         = "consumer-1"
	testObjectStore      = "file-uploads"
	testMaxInFlightLarge = 10
)

// errBadBucket is the message a failed lookup of badBucket produces, asserted
// by more than one test.
const errBadBucket = "failed to get KV bucket bad-bucket: " +
	"failed to create/update KV bucket bad-bucket: bucket creation failed"

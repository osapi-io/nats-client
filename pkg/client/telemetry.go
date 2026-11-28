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
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// instrumentation names the spans and the metrics of this package.
const instrumentation = "github.com/osapi-io/nats-client"

// systemAttr is on each span and each metric: the system that is called.
var systemAttr = attribute.String("db.system.name", "nats")

// telemetry makes the spans and the metrics of the calls to NATS.
type telemetry struct {
	tracer   trace.Tracer
	duration metric.Float64Histogram
}

// newTelemetry returns the telemetry of opts, or nil when Options.Telemetry
// is off. A nil *telemetry makes nothing, so a call needs no check.
func newTelemetry(
	opts *Options,
) (*telemetry, error) {
	if opts == nil || !opts.Telemetry {
		return nil, nil
	}

	tp := opts.TracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	mp := opts.MeterProvider
	if mp == nil {
		mp = otel.GetMeterProvider()
	}

	duration, err := mp.Meter(instrumentation).Float64Histogram(
		"nats.client.operation.duration",
		metric.WithDescription("The duration of each call to NATS."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create the duration metric: %w", err)
	}

	return &telemetry{tracer: tp.Tracer(instrumentation), duration: duration}, nil
}

// op is one call to NATS: its span name, such as kv.get, the attributes of
// its span, and the attributes of its metric. The metric has only the
// attributes with few values, the operation and the bucket or stream, and
// never a key, a subject or an object name.
type op struct {
	name    string
	attrs   []attribute.KeyValue
	metrics []attribute.KeyValue
}

// newOp is an op named name on the bucket or stream namespace, with the
// attributes of its span.
func newOp(
	name string,
	namespace string,
	attrs ...attribute.KeyValue,
) op {
	metrics := []attribute.KeyValue{
		systemAttr,
		attribute.String("db.operation.name", name),
	}
	if namespace != "" {
		metrics = append(metrics, attribute.String("db.namespace", namespace))
	}

	return op{
		name:    name,
		attrs:   append(append([]attribute.KeyValue{}, metrics...), attrs...),
		metrics: metrics,
	}
}

// failed reports whether err is a failure of the call. A key, an object or a
// message that is not there is the answer of a read. A key that exists, or a
// revision that is not the last one, is the answer of KVCreate or KVUpdate:
// the compare that the caller asked for.
func failed(
	err error,
) bool {
	return err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) &&
		!errors.Is(err, jetstream.ErrObjectNotFound) &&
		!errors.Is(err, jetstream.ErrMsgNotFound) &&
		!errors.Is(err, jetstream.ErrKeyExists)
}

// errorType names the type of the innermost error of err, such as
// *jetstream.APIError, for the error.type attribute. The error of a call is
// wrapped in the text of the call, which is not its type.
func errorType(
	err error,
) string {
	for next := errors.Unwrap(err); next != nil; next = errors.Unwrap(err) {
		err = next
	}

	return fmt.Sprintf("%T", err)
}

// start opens the span of o with kind, when ctx is inside a trace. A call
// outside a trace, such as a heartbeat or the setup of a stream at startup,
// makes no span: a root span for each would hide the traces of the requests.
func (t *telemetry) start(
	ctx context.Context,
	o op,
	kind trace.SpanKind,
) (context.Context, trace.Span) {
	if t == nil || !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx, nil
	}

	return t.tracer.Start(ctx, o.name, trace.WithSpanKind(kind), trace.WithAttributes(o.attrs...))
}

// end ends the span of o, and records the duration of o since began. The
// metric is recorded outside a trace too.
func (t *telemetry) end(
	ctx context.Context,
	o op,
	span trace.Span,
	began time.Time,
	err error,
) {
	if t == nil {
		return
	}

	attrs := o.metrics
	if failed(err) {
		attrs = append(append([]attribute.KeyValue{}, attrs...),
			attribute.String("error.type", errorType(err)))
		if span != nil {
			span.SetStatus(codes.Error, err.Error())
		}
	}
	if span != nil {
		span.End()
	}
	t.duration.Record(ctx, time.Since(began).Seconds(), metric.WithAttributes(attrs...))
}

// observe runs fn as the call o, with its span and its metric.
func observe[T any](
	ctx context.Context,
	t *telemetry,
	o op,
	fn func(context.Context) (T, error),
) (T, error) {
	began := time.Now()
	ctx, span := t.start(ctx, o, trace.SpanKindClient)
	v, err := fn(ctx)
	t.end(ctx, o, span, began, err)

	return v, err
}

// observeErr is observe for a call that returns only an error.
func observeErr(
	ctx context.Context,
	t *telemetry,
	o op,
	fn func(context.Context) error,
) error {
	_, err := observe(ctx, t, o, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})

	return err
}

// MessageContext returns ctx with the trace context that a message carries in
// its headers, as Publish and PublishCore put it there. Consume and Subscribe
// give their handler this context already. Use it with
// msg.Headers() of a jetstream.Msg, or msg.Header of a *nats.Msg.
func MessageContext(
	ctx context.Context,
	header nats.Header,
) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(http.Header(header)))
}

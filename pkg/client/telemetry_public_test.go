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

package client_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"

	"github.com/osapi-io/nats-client/pkg/client"
	"github.com/osapi-io/nats-client/pkg/client/mocks"
)

type TelemetryPublicTestSuite struct {
	suite.Suite

	ctrl     *gomock.Controller
	js       *mocks.MockJetStream
	kv       *mocks.MockKeyValue
	spans    *tracetest.SpanRecorder
	reader   *sdkmetric.ManualReader
	tp       *sdktrace.TracerProvider
	client   *client.Client
	previous propagation.TextMapPropagator
}

func (s *TelemetryPublicTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.js = mocks.NewMockJetStream(s.ctrl)
	s.kv = mocks.NewMockKeyValue(s.ctrl)
	s.spans = tracetest.NewSpanRecorder()
	s.reader = sdkmetric.NewManualReader()
	s.tp = sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(s.spans))
	s.client = client.New(slog.Default(), &client.Options{
		Telemetry:      true,
		TracerProvider: s.tp,
		MeterProvider:  sdkmetric.NewMeterProvider(sdkmetric.WithReader(s.reader)),
	})
	client.SetJetStream(s.client, s.js)
	s.previous = otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

func (s *TelemetryPublicTestSuite) TearDownTest() {
	otel.SetTextMapPropagator(s.previous)
}

func (s *TelemetryPublicTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *TelemetryPublicTestSuite) TearDownSubTest() {
	s.TearDownTest()
}

// request starts the span of a request, as a caller of the client does.
func (s *TelemetryPublicTestSuite) request() (context.Context, trace.Span) {
	return s.tp.Tracer("test").Start(context.Background(), "GET /api/x")
}

// attrs returns the attributes as a map.
func attrs(
	kvs []attribute.KeyValue,
) map[string]string {
	out := map[string]string{}
	for _, kv := range kvs {
		out[string(kv.Key)] = kv.Value.String()
	}

	return out
}

// points returns the attributes of each point of the duration metric.
func (s *TelemetryPublicTestSuite) points() []map[string]string {
	var rm metricdata.ResourceMetrics
	s.Require().NoError(s.reader.Collect(context.Background(), &rm))

	var out []map[string]string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			s.Equal("nats.client.operation.duration", m.Name)
			s.Equal("s", m.Unit)
			for _, p := range m.Data.(metricdata.Histogram[float64]).DataPoints {
				out = append(out, attrs(p.Attributes.ToSlice()))
			}
		}
	}

	return out
}

func (s *TelemetryPublicTestSuite) TestCalls() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "a call inside a trace makes a client span with the call's attributes",
			validateFunc: func() {
				s.js.EXPECT().KeyValue(gomock.Any(), "jobs").Return(s.kv, nil)
				s.kv.EXPECT().Put(gomock.Any(), "jobs.1", []byte("v")).
					DoAndReturn(func(ctx context.Context, _ string, _ []byte) (uint64, error) {
						s.True(
							trace.SpanContextFromContext(ctx).IsValid(),
							"the call runs inside its span",
						)
						return 1, nil
					})

				ctx, req := s.request()
				_, err := s.client.KVPut(ctx, "jobs", "jobs.1", []byte("v"))
				req.End()
				s.Require().NoError(err)

				ended := s.spans.Ended()
				s.Require().Len(ended, 2)
				span := ended[0]
				s.Equal("kv.put", span.Name())
				s.Equal(trace.SpanKindClient, span.SpanKind())
				s.Equal(req.SpanContext().SpanID(), span.Parent().SpanID())
				s.Equal(codes.Unset, span.Status().Code)
				s.Equal(map[string]string{
					"db.system.name":    "nats",
					"db.operation.name": "kv.put",
					"db.namespace":      "jobs",
					"nats.kv.key":       "jobs.1",
				}, attrs(span.Attributes()))

				s.Equal([]map[string]string{{
					"db.system.name":    "nats",
					"db.operation.name": "kv.put",
					"db.namespace":      "jobs",
				}}, s.points(), "the metric never has the key")
			},
		},
		{
			name: "a Publish on a client that is not connected has its span and its metric",
			validateFunc: func() {
				client.SetJetStream(s.client, nil)

				ctx, req := s.request()
				err := s.client.Publish(ctx, "jobs.1", []byte("x"))
				req.End()
				s.ErrorIs(err, client.ErrNotConnected)

				ended := s.spans.Ended()
				s.Require().Len(ended, 2)
				s.Equal("nats.publish", ended[0].Name())
				s.Equal(codes.Error, ended[0].Status().Code)
				s.Require().Len(s.points(), 1)
			},
		},
		{
			name: "a call outside a trace makes no span and still records the metric",
			validateFunc: func() {
				s.js.EXPECT().KeyValue(gomock.Any(), "jobs").Return(s.kv, nil)
				s.kv.EXPECT().Delete(gomock.Any(), "k").Return(nil)

				s.Require().NoError(s.client.KVDelete(context.Background(), "jobs", "k"))

				s.Empty(s.spans.Ended(), "background work makes no root span")
				s.Len(s.points(), 1, "the metric is recorded outside a trace too")
			},
		},
		{
			name: "an object store call names its bucket and object",
			validateFunc: func() {
				store := mocks.NewMockObjectStore(s.ctrl)
				s.js.EXPECT().ObjectStore(gomock.Any(), "files").Return(store, nil)
				store.EXPECT().GetBytes(gomock.Any(), "a.conf").Return([]byte("x"), nil)

				ctx, req := s.request()
				_, err := s.client.ObjectGetBytes(ctx, "files", "a.conf")
				req.End()
				s.Require().NoError(err)

				got := attrs(s.spans.Ended()[0].Attributes())
				s.Equal("objectstore.get", got["db.operation.name"])
				s.Equal("files", got["db.namespace"])
				s.Equal("a.conf", got["nats.object.name"])
			},
		},
		{
			name: "with no provider uses the global ones",
			validateFunc: func() {
				previous := otel.GetTracerProvider()
				otel.SetTracerProvider(s.tp)
				defer otel.SetTracerProvider(previous)

				c := client.New(slog.Default(), &client.Options{Telemetry: true})
				client.SetJetStream(c, s.js)
				s.js.EXPECT().KeyValue(gomock.Any(), "jobs").Return(s.kv, nil)
				s.kv.EXPECT().Delete(gomock.Any(), "k").Return(nil)

				ctx, req := s.request()
				s.Require().NoError(c.KVDelete(ctx, "jobs", "k"))
				req.End()
				s.Equal("kv.delete", s.spans.Ended()[0].Name())
			},
		},
		{
			name: "a meter provider that fails turns telemetry off and the calls still work",
			validateFunc: func() {
				c := client.New(slog.Default(), &client.Options{
					Telemetry:      true,
					TracerProvider: s.tp,
					MeterProvider:  brokenMeterProvider{},
				})
				client.SetJetStream(c, s.js)
				s.js.EXPECT().KeyValue(gomock.Any(), "jobs").Return(s.kv, nil)
				s.kv.EXPECT().Delete(gomock.Any(), "k").Return(nil)

				ctx, req := s.request()
				s.Require().NoError(c.KVDelete(ctx, "jobs", "k"), "the calls still work")
				req.End()
				s.Len(s.spans.Ended(), 1, "telemetry is off, so only the request has a span")
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

func (s *TelemetryPublicTestSuite) TestErrors() {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "a failure marks the span and names the type of the error",
			err:      &jetstream.APIError{ErrorCode: 10077, Description: "no space"},
			wantType: "*jetstream.APIError",
		},
		{
			name:     "an error that wraps nothing names its own type",
			err:      errBoom,
			wantType: "*errors.errorString",
		},
		{
			name: "a key that is not there is the answer of a read",
			err:  jetstream.ErrKeyNotFound,
		},
		{
			name: "a message that is not there is the answer of a read",
			err:  jetstream.ErrMsgNotFound,
		},
		{
			name: "a key that exists is the answer of a create",
			err:  jetstream.ErrKeyExists,
		},
		{
			name: "a revision that is not the last is the answer of an update",
			err: &jetstream.APIError{
				Code:        400,
				ErrorCode:   jetstream.JSErrCodeStreamWrongLastSequence,
				Description: "wrong last sequence: 3",
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.js.EXPECT().KeyValue(gomock.Any(), "jobs").Return(s.kv, nil)
			s.kv.EXPECT().Get(gomock.Any(), "k").Return(nil, tc.err)

			ctx, req := s.request()
			_, err := s.client.KVGet(ctx, "jobs", "k")
			req.End()
			s.ErrorIs(err, tc.err)

			span := s.spans.Ended()[0]
			point := s.points()[0]
			if tc.wantType == "" {
				s.Equal(codes.Unset, span.Status().Code)
				s.NotContains(point, "error.type")
				return
			}
			s.Equal(codes.Error, span.Status().Code)
			s.Equal(tc.wantType, point["error.type"])
		})
	}

	s.Run("an error of the handle is in the text of the call", func() {
		s.js.EXPECT().KeyValue(gomock.Any(), "jobs").Return(nil, errBoom)

		_, err := s.client.KVGet(context.Background(), "jobs", "k")
		s.Require().Error(err)
		s.Equal("*errors.errorString", s.points()[0]["error.type"])
	})
}

func (s *TelemetryPublicTestSuite) TestMessageContext() {
	tests := []struct {
		name         string
		validateFunc func()
	}{
		{
			name: "a publish carries the trace context to a subscriber",
			validateFunc: func() {
				nc := mocks.NewMockNATSConnector(s.ctrl)
				s.client.NC = nc

				// The publish puts the trace context in the headers of the message.
				var sent *nats.Msg
				nc.EXPECT().PublishMsg(gomock.Any()).DoAndReturn(func(msg *nats.Msg) error {
					sent = msg
					return nil
				})
				ctx, req := s.request()
				s.Require().NoError(s.client.PublishCore(ctx, "enroll.request", []byte("x")))
				req.End()

				publish := s.spans.Ended()[0]
				s.Equal("nats.publish_core", publish.Name())
				s.Equal(trace.SpanKindProducer, publish.SpanKind())
				s.Equal("enroll.request", attrs(publish.Attributes())["messaging.destination.name"])
				s.NotEmpty(sent.Header.Get("Traceparent"))

				// MessageContext reads it back.
				got := trace.SpanContextFromContext(
					client.MessageContext(context.Background(), sent.Header),
				)
				s.Equal(publish.SpanContext().TraceID(), got.TraceID())
				s.Equal(publish.SpanContext().SpanID(), got.SpanID())

				// A subscriber continues the trace in a consumer span, and its handler
				// gets that span.
				var deliver nats.MsgHandler
				nc.EXPECT().Subscribe("enroll.request", gomock.Any()).
					DoAndReturn(func(_ string, h nats.MsgHandler) (*nats.Subscription, error) {
						deliver = h
						return &nats.Subscription{}, nil
					})
				var handled trace.SpanContext
				_, err := s.client.Subscribe(context.Background(), "enroll.request",
					func(ctx context.Context, _ *nats.Msg) {
						handled = trace.SpanContextFromContext(ctx)
					})
				s.Require().NoError(err)
				deliver(sent)

				receive := s.spans.Ended()[2]
				s.Equal("nats.receive", receive.Name())
				s.Equal(trace.SpanKindConsumer, receive.SpanKind())
				s.Equal(publish.SpanContext().SpanID(), receive.Parent().SpanID())
				s.Equal(receive.SpanContext().SpanID(), handled.SpanID())
			},
		},
		{
			name: "headers with no trace context give a context with no span",
			validateFunc: func() {
				got := trace.SpanContextFromContext(
					client.MessageContext(context.Background(), nats.Header{}),
				)
				s.False(got.IsValid())
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, tc.validateFunc)
	}
}

// brokenMeterProvider makes a meter that cannot make a histogram.
type brokenMeterProvider struct{ noop.MeterProvider }

func (brokenMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return brokenMeter{}
}

type brokenMeter struct{ noop.Meter }

func (brokenMeter) Float64Histogram(
	string,
	...metric.Float64HistogramOption,
) (metric.Float64Histogram, error) {
	return nil, errBoom
}

func TestTelemetryPublicTestSuite(t *testing.T) {
	suite.Run(t, new(TelemetryPublicTestSuite))
}

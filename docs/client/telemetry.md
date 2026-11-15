# Telemetry

The client can make an OpenTelemetry span and a metric for each call to NATS.
Telemetry is off by default. Set `Options.Telemetry` to turn it on.

```go
c := client.New(logger, &client.Options{
    Host:           "localhost",
    Port:           4222,
    Telemetry:      true,
    TracerProvider: tp, // nil uses the global provider
    MeterProvider:  mp, // nil uses the global provider
})
```

## Spans

The client makes a span only inside a trace that the caller started. A call
outside a trace makes no span. For example, a heartbeat or the setup of a stream
at startup makes no span. A root span for each of these calls would hide the
traces of the requests.

| Span                 | Kind     | Made by                                |
| -------------------- | -------- | -------------------------------------- |
| `kv.<operation>`     | Client   | Each `KV` method, for example `kv.get` |
| `objectstore.<op>`   | Client   | Each `Object` method                   |
| `stream.<operation>` | Client   | Each `Stream` and `Consumer` method    |
| `nats.publish`       | Producer | `Publish`                              |
| `nats.publish_core`  | Producer | `PublishCore`                          |
| `nats.receive`       | Consumer | `Subscribe`, for each message          |
| `nats.process`       | Consumer | `Consume`, for each message            |

Each span has `db.system.name` (`nats`), `db.operation.name`, and `db.namespace`
(the bucket or the stream). A span can also have `nats.kv.key`,
`nats.object.name`, or `messaging.destination.name` (the subject).

`Publish` and `PublishCore` put the trace context of their span in the headers
of the message. `Subscribe` and `Consume` continue that trace. Thus the span of
a handler is a child of the span of the publish. The client uses the global text
map propagator for the headers. The application sets it, for example to
`propagation.TraceContext{}`.

## Metric

`nats.client.operation.duration` is a histogram of the duration of each call, in
seconds. The client records it for each call, also outside a trace. Its
attributes are `db.system.name`, `db.operation.name`, and `db.namespace`. A
failed call also has `error.type`, the type of the error, for example
`*jetstream.APIError`.

The metric never has a key, a subject or an object name. These values have no
limit, and a subject can name a host.

## What is not a failure

A key or an object that is not there is the answer of a read. Thus
`jetstream.ErrKeyNotFound` and `jetstream.ErrObjectNotFound` do not mark the
span as an error, and do not add `error.type`. The caller gets the error.

## When the meter provider fails

When the meter provider cannot make the histogram, `New` logs a warning and
leaves telemetry off. The calls to NATS still work.

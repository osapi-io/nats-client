# Messaging

These methods publish messages, and give the messages of a subject or a consumer
to a handler. `Publish` and `Consume` use JetStream. `PublishCore` and
`Subscribe` use core NATS. Core NATS does not keep a message. A message goes
only to the subscribers that listen at that time.

## Methods

| Method                                          | Description                                                     |
| ----------------------------------------------- | --------------------------------------------------------------- |
| `Publish(ctx, subject, data)`                   | Publish a message to a JetStream subject                        |
| `PublishCore(ctx, subject, data)`               | Publish a message with core NATS                                |
| `Subscribe(ctx, subject, handler)`              | Give each core NATS message on a subject to the handler         |
| `Consume(ctx, stream, consumer, handler, opts)` | Give each message of a durable consumer to the handler          |
| `MessageContext(ctx, header)`                   | Return `ctx` with the trace context of the headers of a message |

## Types

| Type                      | Description                                                           |
| ------------------------- | --------------------------------------------------------------------- |
| `MsgHandler`              | `func(ctx context.Context, msg *nats.Msg)`                            |
| `JetStreamMessageHandler` | `func(ctx context.Context, msg jetstream.Msg) error`                  |
| `ConsumeOptions`          | The maximum count of messages that `Consume` handles at the same time |

## Usage

```go
err := c.Publish(ctx, "jobs.query.web-01", []byte(jobID))

err = c.Consume(ctx, "JOBS", "jobs-agent",
    func(ctx context.Context, msg jetstream.Msg) error {
        return process(ctx, msg.Data())
    },
    &client.ConsumeOptions{MaxInFlight: 10},
)

sub, err := c.Subscribe(ctx, "enroll.response.web-01",
    func(ctx context.Context, msg *nats.Msg) {
        handle(ctx, msg.Data)
    })
defer sub.Unsubscribe()
```

## Behavior to know

- The headers of each published message have the trace context of `ctx`. The
  `ctx` of a handler has the trace context of the message. Thus the work of a
  handler is in the trace of the request that published the message. See
  [Telemetry](telemetry.md).
- `Consume` runs up to `MaxInFlight` handlers at the same time. The default is
  `DefaultMaxInFlight`, 10. It fetches only as many messages as it has free
  handlers for, so a message that it fetched does not wait for its `AckWait`.
- `Consume` acknowledges a message when the handler returns nil. When the
  handler returns an error or panics, `Consume` sends a negative
  acknowledgement. Then JetStream sends the message again, after the `BackOff`
  of the consumer if it has one. When the error wraps `ErrTerminate`, `Consume`
  ends the message, and JetStream does not send it again.
- When a fetch fails, `Consume` tries again after 100 ms. The wait doubles after
  each failure, up to 5 s. When the connection is closed, or the consumer or its
  stream is gone, `Consume` returns the error, so the caller can make the
  consumer again.
- `Consume` runs until `ctx` ends, and then returns `ctx.Err()`. A wait for a
  message also stops when `ctx` ends. Thus a shutdown does not wait for the pull
  request to expire. `Consume` returns only after each handler that it started
  has returned.
- A panic in a `Subscribe` handler does not stop the process. The span of the
  message records it as an error.
- `Subscribe` with a `ctx` that ended subscribes nothing. The end of `ctx`
  unsubscribes. An unsubscribe by hand also stops the watch of `ctx`, through
  the closed handler of the subscription. A caller that sets its own closed
  handler replaces that one, and the watch then lasts until `ctx` ends.
- A `Publish` on a client that is not connected has its span and its metric, as
  the other calls do.

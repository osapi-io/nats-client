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

| Type                      | Description                                                  |
| ------------------------- | ------------------------------------------------------------ |
| `MsgHandler`              | `func(ctx context.Context, msg *nats.Msg)`                   |
| `JetStreamMessageHandler` | `func(ctx context.Context, msg jetstream.Msg) error`         |
| `ConsumeOptions`          | The queue group, and the maximum count of messages in flight |

## Usage

```go
err := c.Publish(ctx, "jobs.query.web-01", []byte(jobID))

err = c.Consume(ctx, "JOBS", "jobs-agent",
    func(ctx context.Context, msg jetstream.Msg) error {
        return process(ctx, msg.Data())
    },
    &client.ConsumeOptions{QueueGroup: "job-agents", MaxInFlight: 10},
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
- `Consume` acknowledges a message when the handler returns nil. When the
  handler returns an error or panics, `Consume` does not acknowledge the
  message. Then JetStream sends it again.
- `Consume` runs until `ctx` ends, and then returns `ctx.Err()`.

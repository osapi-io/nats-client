# Core NATS

Publish and subscribe with core NATS, not JetStream. Core NATS does not store
messages. A message goes only to the subscribers that listen at that time.

## Methods

| Method                        | Description                                               |
| ----------------------------- | --------------------------------------------------------- |
| `PublishCore(subject, data)`  | Publish a message with core NATS                          |
| `Subscribe(subject, handler)` | Subscribe to a subject; the handler receives each message |

`PublishCore` does not add OpenTelemetry trace headers, and it does not need a
stream. Use it for messages that need no storage, for example an enrollment
reply. Use `Publish` for a message that a stream must keep.

## Usage

```go
sub, err := c.Subscribe("enroll.request", func(msg *nats.Msg) {
    handle(msg.Data)
})
defer sub.Unsubscribe()

err = c.PublishCore("enroll.reply", reply)
```

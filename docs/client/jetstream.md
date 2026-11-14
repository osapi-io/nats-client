# JetStream

These methods create and update JetStream streams. They also publish messages to
the streams.

## Methods

| Method                                         | Description                                          |
| ---------------------------------------------- | ---------------------------------------------------- |
| `CreateOrUpdateStreamWithConfig(ctx, config)`  | Create or update a JetStream stream                  |
| `Publish(ctx, subject, data)`                  | Publish a message to a JetStream subject             |
| `PublishWithHeaders(ctx, subject, hdrs, data)` | Publish with NATS headers, for example trace context |

## Usage

```go
err := c.CreateOrUpdateStreamWithConfig(ctx, jetstream.StreamConfig{
    Name:     "JOBS",
    Subjects: []string{"jobs.>"},
    MaxAge:   24 * time.Hour,
    Storage:  jetstream.FileStorage,
})
```

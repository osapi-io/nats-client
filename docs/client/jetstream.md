# JetStream

Create and manage JetStream streams.

## Methods

| Method                                        | Description                                            |
| --------------------------------------------- | ------------------------------------------------------ |
| `CreateOrUpdateStreamWithConfig(ctx, config)` | Create or update a JetStream stream                    |
| `Publish(ctx, subject, data)`                 | Publish a message to a JetStream subject               |
| `PublishWithHeaders(ctx, subject, hdrs, data)` | Publish with NATS headers (e.g., trace propagation)   |

## Usage

```go
err := c.CreateOrUpdateStreamWithConfig(ctx, jetstream.StreamConfig{
    Name:     "JOBS",
    Subjects: []string{"jobs.>"},
    MaxAge:   24 * time.Hour,
    Storage:  jetstream.FileStorage,
})
```

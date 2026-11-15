# Streams

These methods create JetStream streams and their durable consumers, and read the
state and the messages of a stream. Each method takes the name of the stream.

## Methods

| Method                                     | Description                                                    |
| ------------------------------------------ | -------------------------------------------------------------- |
| `StreamCreateOrUpdate(ctx, config)`        | Create a stream, or change it to agree with the configuration  |
| `ConsumerCreateOrUpdate(ctx, stream, cfg)` | Create a durable consumer on a stream, or change it            |
| `StreamInfo(ctx, name)`                    | Return the configuration and the current state of a stream     |
| `StreamLastMsg(ctx, name, subject)`        | Return the last message on a subject                           |
| `StreamRead(ctx, name, from, count)`       | Read up to `count` messages in order, from the sequence `from` |

## Usage

```go
err := c.StreamCreateOrUpdate(ctx, jetstream.StreamConfig{
    Name:     "JOBS",
    Subjects: []string{"jobs.>"},
    MaxAge:   24 * time.Hour,
    Storage:  jetstream.FileStorage,
})

err = c.ConsumerCreateOrUpdate(ctx, "JOBS", jetstream.ConsumerConfig{
    Durable:   "jobs-agent",
    AckPolicy: jetstream.AckExplicitPolicy,
})

// Read 100 messages from sequence 1, and wait 2 seconds at most.
readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()
msgs, err := c.StreamRead(readCtx, "AUDIT", 1, 100)
```

## Behavior to know

- `StreamRead` waits for its messages until the deadline of `ctx`. When `ctx`
  has no deadline, it waits 5 seconds. A sequence of 0 reads from the first
  message.
- When the read stops with an error, `StreamRead` returns the messages that it
  read and the error.
- `StreamInfo` asks the server each time. Thus the state is current.
- A stream with a different storage type stays as it is, and
  `StreamCreateOrUpdate` returns no error.

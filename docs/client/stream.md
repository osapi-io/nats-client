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

// Read up to 100 messages from sequence 1.
msgs, err := c.StreamRead(ctx, "AUDIT", 1, 100)
```

## Behavior to know

- `StreamRead` returns the messages that the stream has now, and does not wait
  for more. Thus a read of more messages than the stream has returns at once,
  with the messages that it has. A sequence of 0 reads from the first message.
  A `count` less than 1 is an error.
- `StreamRead` deletes the consumer that it read with, so a read leaves no
  consumer on the server.
- When the read stops with an error, `StreamRead` returns the messages that it
  read and the error.
- `StreamInfo` asks the server each time. Thus the state is current.
- A stream with a different storage type stays as it is, and
  `StreamCreateOrUpdate` returns no error.

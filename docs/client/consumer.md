# Consumer helpers

Create JetStream consumers, and use them to process messages.

## Methods

| Method                                                          | Description                                  |
| --------------------------------------------------------------- | -------------------------------------------- |
| `ConsumeMessages(ctx, stream, consumer, handler, opts)`         | Subscribe to a consumer and process messages |
| `CreateOrUpdateConsumerWithConfig(ctx, stream, consumerConfig)` | Create or update a durable consumer          |

## Types

| Type                      | Description                                                  |
| ------------------------- | ------------------------------------------------------------ |
| `JetStreamMessageHandler` | `func(msg jetstream.Msg) error` callback                     |
| `ConsumeOptions`          | The queue group and the maximum number of messages in flight |

## Usage

```go
handler := func(msg jetstream.Msg) error {
    fmt.Println("received:", string(msg.Data()))
    return msg.Ack()
}

err := c.ConsumeMessages(ctx, "JOBS", "jobs-agent", handler, &client.ConsumeOptions{
    QueueGroup:  "job-agents",
    MaxInFlight: 10,
})
```

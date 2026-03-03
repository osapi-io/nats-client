# Consumer Helpers

Create and manage JetStream consumers for message processing.

## Methods

| Method                                                          | Description                                     |
| --------------------------------------------------------------- | ----------------------------------------------- |
| `ConsumeMessages(ctx, stream, consumer, handler, opts)`         | Subscribe to a consumer and process messages    |
| `CreateOrUpdateConsumerWithConfig(ctx, stream, consumerConfig)` | Create or update a durable consumer             |

## Types

| Type                      | Description                                      |
| ------------------------- | ------------------------------------------------ |
| `JetStreamMessageHandler` | `func(msg jetstream.Msg) error` callback         |
| `ConsumeOptions`          | Queue group and max in-flight configuration      |

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

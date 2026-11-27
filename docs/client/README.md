# NATS client

The `client` package gives a NATS JetStream client. The client has connection
management, KV buckets, Object Store buckets, streams, messaging, and
OpenTelemetry spans and metrics. To make a client, call `New()`. Then call
`Connect()` to open the connection.

Each method takes a `context.Context` first, and the name of the bucket or the
stream. The methods return the types of `nats.go` and `jetstream`. The package
declares no interface for its own methods. A program that uses the client
declares the interface that it needs, in the package that uses it.

## Quick start

```go
c := client.New(logger, &client.Options{
    Host: "localhost",
    Port: 4222,
    Name: "my-app",
})

if err := c.Connect(ctx); err != nil {
    log.Fatal(err)
}
```

## Features

| Feature                        | Description                                      | Source           |
| ------------------------------ | ------------------------------------------------ | ---------------- |
| [Connection](connection.md)    | Connect to NATS with an authentication type      | `connect.go`     |
| [Key-value](kv.md)             | Create KV buckets, and read and write their keys | `kv.go`          |
| [Object Store](objectstore.md) | Store large blobs that NATS divides into chunks  | `objectstore.go` |
| [Streams](stream.md)           | Create streams and consumers, and read a stream  | `stream.go`      |
| [Messaging](messaging.md)      | Publish, subscribe and consume                   | `messaging.go`   |
| [Telemetry](telemetry.md)      | OpenTelemetry spans and metrics for each call    | `telemetry.go`   |

## Authentication

| Type              | Description                             | Constant       |
| ----------------- | --------------------------------------- | -------------- |
| No auth           | Connect without authentication          | `NoAuth`       |
| Username/password | Username and password                   | `UserPassAuth` |
| NKey              | An Ed25519 public and private key pair  | `NKeyAuth`     |
| Credentials       | A user JWT and an NKey seed in one file | `CredsAuth`    |

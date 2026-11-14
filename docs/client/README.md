# NATS client

The `client` package gives a NATS JetStream client. The client has connection
management, KV stores, KV-backed streams, consumer helpers, and object stores.
To make a client, call `New()`. Then call `Connect()` to open the connection.

## Quick start

```go
c := client.New(logger, &client.Options{
    Host: "localhost",
    Port: 4222,
    Name: "my-app",
})

if err := c.Connect(); err != nil {
    log.Fatal(err)
}
```

## Features

| Feature                             | Description                                   | Source           |
| ----------------------------------- | --------------------------------------------- | ---------------- |
| [`Connection`](connection.md)       | Connect to NATS with authentication modes     | `connect.go`     |
| [`JetStream`](jetstream.md)         | Create and update streams, and publish        | `jetstream.go`   |
| [`Key-Value`](kv.md)                | Create, read, update and delete in KV         | `kv.go`          |
| [`KV-Backed Streams`](kv-stream.md) | Write to KV and publish a notification        | `kv_stream.go`   |
| [`Consumer`](consumer.md)           | Process messages with JetStream consumers     | `consumer.go`    |
| [`Object Store`](objectstore.md)    | Store large blobs that NATS divides in chunks | `objectstore.go` |

## Authentication

| Type          | Description                            | Constant       |
| ------------- | -------------------------------------- | -------------- |
| No auth       | Connect without authentication         | `NoAuth`       |
| Username/pass | Username and password                  | `UserPassAuth` |
| NKey          | An Ed25519 public and private key pair | `NKeyAuth`     |

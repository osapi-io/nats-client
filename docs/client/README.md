# NATS Client

The `client` package provides a NATS JetStream client with connection
management, KV stores, KV-backed streams, and consumer helpers. Create a client
with `New()` and call `Connect()` to establish the connection.

## Quick Start

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

| Feature                                       | Description                                  | Source             |
| --------------------------------------------- | -------------------------------------------- | ------------------ |
| [`Connection`](connection.md)                 | Connect to NATS with configurable auth modes | `connect.go`       |
| [`JetStream`](jetstream.md)                   | Create and manage JetStream streams          | `jetstream.go`     |
| [`Key-Value`](kv.md)                          | CRUD operations on NATS KV buckets           | `kv.go`            |
| [`KV-Backed Streams`](kv-stream.md)           | KV storage with stream notifications         | `kv_stream.go`     |
| [`Consumer`](consumer.md)                     | JetStream consumer message processing        | `consumer.go`      |
| [`Object Store`](objectstore.md)              | Large blob storage with automatic chunking   | `objectstore.go`   |

## Authentication

| Type           | Description                         | Constant       |
| -------------- | ----------------------------------- | -------------- |
| No auth        | Connect without authentication      | `NoAuth`       |
| Username/pass  | Username and password               | `UserPassAuth` |
| NKey           | Ed25519 public-private key pairs    | `NKeyAuth`     |

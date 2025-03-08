# NATS Streaming Without Authentication

This example demonstrates how to interact with NATS by creating streams and
consumers withouth Auth.

## Usage

Run the client:

```bash
$ go run main.go
```

Get stream information:

```bats
$ nats s info STREAM2
```

Get consumer information:

```bash
$ nats c info STREAM2 consumer3
$ nats c info STREAM2 consumer4
```

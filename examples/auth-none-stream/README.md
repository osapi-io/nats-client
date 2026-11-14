# NATS streaming without authentication

This example creates streams and consumers in NATS. It uses no authentication.

## Usage

Run the client:

```bash
$ go run main.go
```

Get the stream information:

```bats
$ nats s info STREAM2
```

Get the consumer information:

```bash
$ nats c info STREAM2 consumer3
$ nats c info STREAM2 consumer4
```

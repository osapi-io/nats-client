# NATS streaming with NKEY authentication

This example creates streams and consumers in NATS. It uses [NKey Auth].

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

[nkey auth]: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/nkey_auth

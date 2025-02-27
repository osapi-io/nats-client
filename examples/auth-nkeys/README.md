# Authenticating with an NKey

An example NATS client using [NKey Auth][].

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

[NKey Auth]: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/nkey_auth

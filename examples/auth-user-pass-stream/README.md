# Authenticating with a User and Password

An example NATS client using [User and Password Auth][].

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

[User and Password Auth]: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/username_password

# NATS streaming with a credentials file

This example creates a stream and its consumers in NATS. It connects with
`CredsAuth`: a credentials file that holds a user JWT and the NKEY seed of the
user. Use this mode with a NATS server in operator mode ([JWT auth]).

## Usage

Put the credentials file of a user in `.creds/service1.creds`. The account of
the user must have JetStream. `nsc` makes the file:

```bash
$ nsc generate creds --account APP --name service1 > .creds/service1.creds
```

Run the client:

```bash
$ go run main.go
```

Get the stream information:

```bash
$ nats s info STREAM3
```

Get the consumer information:

```bash
$ nats c info STREAM3 consumer5
$ nats c info STREAM3 consumer6
```

[jwt auth]: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/jwt

<p align="center">
  <picture>
    <source srcset="asset/logo-dark.svg" media="(prefers-color-scheme: dark)">
    <source srcset="asset/logo-light.svg" media="(prefers-color-scheme: light)">
    <img src="asset/logo-dark.svg" alt="nats-client" width="582">
  </picture>
</p>

<p align="center">A Go package that connects to a NATS server and operates on it.</p>

<p align="center">
  <a href="https://github.com/osapi-io/nats-client/releases/latest"><img alt="release" src="https://img.shields.io/github/release/osapi-io/nats-client.svg?style=for-the-badge"></a>
  <a href="https://codecov.io/gh/osapi-io/nats-cllient"><img alt="codecov" src="https://img.shields.io/codecov/c/github/osapi-io/nats-client?token=8RICN0QCTT&style=for-the-badge"></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-brightgreen.svg?style=for-the-badge"></a>
  <a href="https://github.com/osapi-io/nats-client/actions/workflows/go.yml"><img alt="build" src="https://img.shields.io/github/actions/workflow/status/osapi-io/nats-client/go.yml?style=for-the-badge"></a>
  <a href="https://github.com/goreleaser"><img alt="powered by" src="https://img.shields.io/badge/powered%20by-goreleaser-green.svg?style=for-the-badge"></a>
  <a href="https://conventionalcommits.org"><img alt="conventional commits" src="https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg?style=for-the-badge"></a>
  <a href="https://nats.io"><img alt="nats" src="https://img.shields.io/badge/NATS-27AAE1?style=for-the-badge&logo=natsdotio&logoColor=white"></a>
  <a href="https://just.systems"><img alt="built with just" src="https://img.shields.io/badge/Built_with-Just-black?style=for-the-badge&logo=just&logoColor=white"></a>
  <img alt="gitHub commit activity" src="https://img.shields.io/github/commit-activity/m/osapi-io/nats-client?style=for-the-badge">
  <a href="https://pkg.go.dev/github.com/osapi-io/nats-client/pkg/client"><img alt="go reference" src="https://img.shields.io/badge/go-reference-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
</p>

<p align="center">
<b>One client for the connection and for JetStream.</b>
</p>

<p align="center">
One client and one constructor give connection management, JetStream, KV
buckets and consumers. The client has three authentication modes. Its
create-or-update helpers make the setup idempotent.
</p>

## Install

```bash
go get github.com/osapi-io/nats-client
```

## Features

The [client docs](docs/client/README.md) give a quick start, the authentication
modes, and a reference for each feature.

| Feature               | Description                                            | Docs                               | Source                                        |
| --------------------- | ------------------------------------------------------ | ---------------------------------- | --------------------------------------------- |
| Connection management | Connect to NATS with options and authentication modes  | [docs](docs/client/connection.md)  | [`connect.go`](pkg/client/connect.go)         |
| JetStream             | Create and update JetStream streams, and publish       | [docs](docs/client/jetstream.md)   | [`jetstream.go`](pkg/client/jetstream.go)     |
| Key-Value stores      | Create, read, update and delete in NATS KV buckets     | [docs](docs/client/kv.md)          | [`kv.go`](pkg/client/kv.go)                   |
| KV-backed streams     | Write to a KV bucket and publish a stream notification | [docs](docs/client/kv-stream.md)   | [`kv_stream.go`](pkg/client/kv_stream.go)     |
| Consumer helpers      | Process messages with JetStream consumers              | [docs](docs/client/consumer.md)    | [`consumer.go`](pkg/client/consumer.go)       |
| Object Store          | Store large blobs, which NATS divides into chunks      | [docs](docs/client/objectstore.md) | [`objectstore.go`](pkg/client/objectstore.go) |

## Examples

Each example is a standalone Go program. You can read it and run it.

| Example                                                         | What it shows                                          |
| --------------------------------------------------------------- | ------------------------------------------------------ |
| [auth-none-stream](examples/auth-none-stream/main.go)           | Stream publish and subscribe, no authentication        |
| [auth-none-kv](examples/auth-none-kv/main.go)                   | KV store operations, no authentication                 |
| [auth-user-pass-stream](examples/auth-user-pass-stream/main.go) | Stream with username and password authentication       |
| [auth-nkeys-stream](examples/auth-nkeys-stream/main.go)         | Stream with NKey authentication                        |
| [workqueue-stream](examples/workqueue-stream/main.go)           | Work queue with one durable consumer for each priority |

## Documentation

- [Package documentation] on pkg.go.dev. This is the API reference.

## Contributing

The [Contributing](CONTRIBUTING.md) guide gives the prerequisites, the setup,
the conventions, and the PR workflow.

## License

The [MIT] License.

[mit]: LICENSE
[package documentation]: https://pkg.go.dev/github.com/osapi-io/nats-client/pkg/client

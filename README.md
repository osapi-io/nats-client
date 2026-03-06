[![release](https://img.shields.io/github/release/osapi-io/nats-client.svg?style=for-the-badge)](https://github.com/osapi-io/nats-client/releases/latest)
[![codecov](https://img.shields.io/codecov/c/github/osapi-io/nats-client?token=8RICN0QCTT&style=for-the-badge)](https://codecov.io/gh/osapi-io/nats-cllient)
[![go report card](https://goreportcard.com/badge/github.com/osapi-io/nats-client?style=for-the-badge)](https://goreportcard.com/report/github.com/osapi-io/nats-client)
[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=for-the-badge)](LICENSE)
[![build](https://img.shields.io/github/actions/workflow/status/osapi-io/nats-client/go.yml?style=for-the-badge)](https://github.com/osapi-io/nats-client/actions/workflows/go.yml)
[![powered by](https://img.shields.io/badge/powered%20by-goreleaser-green.svg?style=for-the-badge)](https://github.com/goreleaser)
[![conventional commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg?style=for-the-badge)](https://conventionalcommits.org)
[![nats](https://img.shields.io/badge/NATS-27AAE1?style=for-the-badge&logo=natsdotio&logoColor=white)](https://nats.io)
[![built with just](https://img.shields.io/badge/Built_with-Just-black?style=for-the-badge&logo=just&logoColor=white)](https://just.systems)
![gitHub commit activity](https://img.shields.io/github/commit-activity/m/osapi-io/nats-client?style=for-the-badge)

# NATS Client

A Go package for connecting to and interacting with a NATS server.

## 📦 Install

```bash
go get github.com/osapi-io/nats-client
```

## ✨ Features

See the [client docs](docs/client/README.md) for quick start, authentication,
and per-feature reference.

| Feature               | Description                                              | Docs                                       | Source                                                  |
| --------------------- | -------------------------------------------------------- | ------------------------------------------ | ------------------------------------------------------- |
| Connection management | Connect to NATS with configurable options and auth modes | [docs](docs/client/connection.md)          | [`connect.go`](pkg/client/connect.go)                   |
| JetStream             | Create and manage JetStream streams                      | [docs](docs/client/jetstream.md)           | [`jetstream.go`](pkg/client/jetstream.go)               |
| Key-Value stores      | CRUD operations on NATS KV buckets                       | [docs](docs/client/kv.md)                  | [`kv.go`](pkg/client/kv.go)                             |
| KV-backed streams     | KV storage with stream notifications                     | [docs](docs/client/kv-stream.md)           | [`kv_stream.go`](pkg/client/kv_stream.go)               |
| Consumer helpers      | JetStream consumer message processing                    | [docs](docs/client/consumer.md)            | [`consumer.go`](pkg/client/consumer.go)                 |
| Object Store          | Large blob storage with automatic chunking               | [docs](docs/client/objectstore.md)         | [`objectstore.go`](pkg/client/objectstore.go)           |

## 📋 Examples

Each example is a standalone Go program you can read and run.

| Example                                                    | What it shows                                      |
| ---------------------------------------------------------- | -------------------------------------------------- |
| [auth-none-stream](examples/auth-none-stream/main.go)     | Stream publish/subscribe without authentication    |
| [auth-none-kv](examples/auth-none-kv/main.go)             | KV store operations without authentication         |
| [auth-user-pass-stream](examples/auth-user-pass-stream/main.go) | Stream with username/password authentication |
| [auth-nkeys-stream](examples/auth-nkeys-stream/main.go)   | Stream with NKey authentication                    |
| [workqueue-stream](examples/workqueue-stream/main.go)     | Work queue pattern with consumer groups            |

## 📖 Documentation

See the [generated documentation][] for package-level API details.

## 🤝 Contributing

See the [Development](docs/development.md) guide for prerequisites, setup,
and conventions. See the [Contributing](docs/contributing.md) guide before
submitting a PR.

## 📄 License

The [MIT][] License.

[generated documentation]: docs/gen/
[MIT]: LICENSE

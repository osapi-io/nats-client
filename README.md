[![release](https://img.shields.io/github/release/osapi-io/nats-client.svg?style=for-the-badge)](https://github.com/osapi-io/nats-client/releases/latest)
[![codecov](https://img.shields.io/codecov/c/github/osapi-io/nats-client?token=8RICN0QCTT&style=for-the-badge)](https://codecov.io/gh/osapi-io/nats-cllient)
[![go report card](https://goreportcard.com/badge/github.com/osapi-io/nats-client?style=for-the-badge)](https://goreportcard.com/report/github.com/osapi-io/nats-client)
[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=for-the-badge)](LICENSE)
[![build](https://img.shields.io/github/actions/workflow/status/osapi-io/nats-client/go.yml?style=for-the-badge)](https://github.com/osapi-io/nats-client/actions/workflows/go.yml)
[![powered by](https://img.shields.io/badge/powered%20by-goreleaser-green.svg?style=for-the-badge)](https://github.com/goreleaser)
[![conventional commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg?style=for-the-badge)](https://conventionalcommits.org)
![gitHub commit activity](https://img.shields.io/github/commit-activity/m/osapi-io/nats-client?style=for-the-badge)

# NATS Client

A Go package for connecting to and interacting with a NATS server.

## Usage

https://github.com/osapi-io/nats-client/blob/328fbaeed5315b5d4a7db6975bdc75c8d657e567/examples/auth-none-stream/main.go#L21-L87

See the [examples][] section for additional use cases.

## Documentation

See the [generated documentation][] for details on available packages and functions.

## Development

Fetch shared justfiles:

```bash
just fetch
```

Install dependencies:

```bash
just deps
```

Run all tests:

```bash
just test
```

Auto format code:

```bash
just go::fmt
```

List available recipes:

```bash
just --list
```

## License

The [MIT][] License.

[examples]: examples/
[generated documentation]: docs/gen/
[MIT]: LICENSE

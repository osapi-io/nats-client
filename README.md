[![codecov](https://img.shields.io/codecov/c/github/osapi-io/nats-client?token=8RICN0QCTT&style=for-the-badge)](https://codecov.io/gh/osapi-io/nats-cllient)
[![go report card](https://goreportcard.com/badge/github.com/osapi-io/nats-client?style=for-the-badge)](https://goreportcard.com/report/github.com/osapi-io/nats-client)
[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=for-the-badge)](LICENSE)
[![conventional commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg?style=for-the-badge)](https://conventionalcommits.org)
![gitHub commit activity](https://img.shields.io/github/commit-activity/m/osapi-io/nats-client?style=for-the-badge)

# NATS Client

A Go package for connecting to and interacting with a NATS server.

## Usage

```golang:examples/auth-none-stream/main.go
```

See the [examples][] section for additional use cases.

## Testing

Enable [Remote Taskfile][] feature.

```bash
export TASK_X_REMOTE_TASKFILES=1
```

Install dependencies:

```bash
$ task go:deps
```

To execute tests:

```bash
$ task go:test
```

Auto format code:

```bash
$ task go:fmt
```

List helpful targets:

```bash
$ task
```

## License

The [MIT][] License.

[examples]: examples/
[Remote Taskfile]: https://taskfile.dev/experiments/remote-taskfiles/
[MIT]: LICENSE

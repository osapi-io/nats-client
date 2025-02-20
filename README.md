[![codecov](https://img.shields.io/codecov/c/github/osapi-io/nats-client?token=8RICN0QCTT&style=for-the-badge)](https://codecov.io/gh/osapi-io/nats-cllient)
[![go report card](https://goreportcard.com/badge/github.com/osapi-io/nats-client?style=for-the-badge)](https://goreportcard.com/report/github.com/osapi-io/nats-client)
[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=for-the-badge)](LICENSE)
[![conventional commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg?style=for-the-badge)](https://conventionalcommits.org)
![gitHub commit activity](https://img.shields.io/github/commit-activity/m/osapi-io/nats-client?style=for-the-badge)

# NATS Client

A Go package for connecting to and interacting with a NATS server.

## Usage

```golang
package main

import (
	"time"

	"github.com/nats-io/nats.go"
	"github.com/osapi-io/nats-client/pkg/client"
)

func main() {
  host := "localhost"
  port := 4222
	logger := getLogger(debug)

	jsOpts := &client.ClientOptions{
		Host: s.Opts.Host,
		Port: s.Opts.Port,
		Auth: client.AuthOptions{
			AuthType: client.NoAuth,
		},
	}

	js, err := client.NewJetStreamContext(host, port)
	if err != nil {
		logger.Error("failed to create jetstream context", "error", err)
		os.Exit(1)
	}

	streamOpts1 := &client.StreamConfig{
		StreamConfig: &nats.StreamConfig{
			Name:     "TASK_QUEUE",
			Subjects: []string{"tasks.*"},
			Storage:  nats.FileStorage,
			Replicas: 1,
		},
		Consumers: []*client.ConsumerConfig{
			{
				ConsumerConfig: &nats.ConsumerConfig{
					Durable:       "worker1",
					AckPolicy:     nats.AckExplicitPolicy,
					MaxAckPending: 10,
					AckWait:       30 * time.Second,
				},
			},
			{
				ConsumerConfig: &nats.ConsumerConfig{
					Durable:       "worker2",
					AckPolicy:     nats.AckExplicitPolicy,
					MaxAckPending: 10,
					AckWait:       30 * time.Second,
				},
			},
		},
	}

	streamOpts2 := &client.StreamConfig{
		StreamConfig: &nats.StreamConfig{
			Name:     "STREAM2",
			Subjects: []string{"stream2.*"},
			Storage:  nats.FileStorage,
			Replicas: 1,
		},
		Consumers: []*client.ConsumerConfig{
			{
				ConsumerConfig: &nats.ConsumerConfig{
					Durable:    "consumer3",
					AckPolicy:  nats.AckExplicitPolicy,
					MaxDeliver: 5,
					AckWait:    30 * time.Second,
				},
			},
			{
				ConsumerConfig: &nats.ConsumerConfig{
					Durable:    "consumer4",
					AckPolicy:  nats.AckExplicitPolicy,
					MaxDeliver: 5,
					AckWait:    30 * time.Second,
				},
			},
		},
	}

	c := client.New(logger)
	if err := c.SetupJetStream(js, streamOpts1, streamOpts2); err != nil {
		logger.Error("failed setting up jetstream", "error", err)
		os.Exit(1)
	}
}
```

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

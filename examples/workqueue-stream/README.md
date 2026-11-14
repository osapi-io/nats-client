# Workqueue stream

A worker that receives messages from a work queue stream through a consumer.

This example is adapted from [14-jetstream-workqueues].

## Usage

Publish to the stream:

```bash
$ nats bench --pub 10 jobs.high.my_job_id --syncpub --msgs 5000
$ nats bench --pub 10 jobs.low.my_job_id --syncpub --msgs 5000
```

Run the workers:

```bash
# Run 10 high priority workers
$ seq 10 | xargs -P10 -I {} go run main.go high {}

# Run 2 low priority workers
$ seq 2 | xargs -P2 -I {} go run main.go low {}
```

Get the stream report:

```bash
$ nats stream report
```

[14-jetstream-workqueues]: https://github.com/synadia-io/rethink_connectivity/tree/main/14-jetstream-workqueues

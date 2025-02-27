# Workqueue Stream

A worker consumer to interact with a work queue stream.

Adapted from [14-jetstream-workqueues][].

## Usage

Publishing to the stream:

```bash
$ nats bench --pub 10 jobs.high.my_job_id --syncpub --msgs 5000
$ nats bench --pub 10 jobs.low.my_job_id --syncpub --msgs 5000
```

Running the worker:

```bash
# Run 10 high priority workers
$ seq 10 | xargs -P10 -I {} go run main.go high {}

# Run 2 low priority workers
$ seq 2 | xargs -P2 -I {} go run main.go low {}
```

Get Stream Report:

```bash
$ nats stream report
```

[14-jetstream-workqueues]: https://github.com/synadia-io/rethink_connectivity/tree/main/14-jetstream-workqueues

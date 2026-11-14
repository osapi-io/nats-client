# KV-backed streams

`KVPutAndPublish` writes data to a KV bucket. Then it publishes a stream
notification. One call does the two steps. It is for a workflow system that must
keep the data and also send a notification.

## Methods

| Method                                             | Description                              |
| -------------------------------------------------- | ---------------------------------------- |
| `KVPutAndPublish(ctx, bucket, key, data, subject)` | Write to KV, then publish a notification |

## Usage

```go
revision, err := c.KVPutAndPublish(
    ctx,
    "job-queue",
    "job-123",
    jobData,
    "jobs.query._any",
)
```

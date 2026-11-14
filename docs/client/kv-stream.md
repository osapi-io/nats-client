# KV-backed streams

Write data to a KV bucket and publish a stream notification in one operation.
Use it when a workflow system must keep the data and must also send an event.

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

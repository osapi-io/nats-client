# KV-Backed Streams

Store data in KV and send a stream notification in a single operation. Useful
for workflow systems that need persistent storage plus event notification.

## Methods

| Method                                             | Description                           |
| -------------------------------------------------- | ------------------------------------- |
| `KVPutAndPublish(ctx, bucket, key, data, subject)` | Store in KV then publish notification |

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

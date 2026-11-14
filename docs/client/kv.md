# Key-value buckets

These methods create and update NATS KV buckets. They also read, write and
delete the keys in a bucket.

## Methods

| Method                                          | Description                                                 |
| ----------------------------------------------- | ----------------------------------------------------------- |
| `CreateOrUpdateKVBucket(ctx, name)`             | Create or update a KV bucket with the default configuration |
| `CreateOrUpdateKVBucketWithConfig(ctx, config)` | Create or update a KV bucket with a configuration           |
| `KVGet(ctx, bucket, key)`                       | Read a value from a KV bucket                               |
| `KVPut(ctx, bucket, key, data)`                 | Write a value to a KV bucket                                |
| `KVDelete(ctx, bucket, key)`                    | Delete a key from a KV bucket                               |
| `KVKeys(ctx, bucket)`                           | List all keys in a KV bucket                                |

## Usage

```go
kv, err := c.CreateOrUpdateKVBucketWithConfig(ctx, jetstream.KeyValueConfig{
    Bucket:   "job-queue",
    TTL:      1 * time.Hour,
    MaxBytes: 100 * 1024 * 1024,
    Storage:  jetstream.FileStorage,
})

revision, err := kv.Put(ctx, "job-123", jobData)
entry, err := kv.Get(ctx, "job-123")
```

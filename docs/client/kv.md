# Key-value buckets

These methods create KV buckets, and read, write and delete the keys in them.
Each method takes the name of the bucket. The client keeps a handle for each
bucket that it uses. Thus it asks the server for the handle one time only.

## Methods

| Method                                        | Description                                                           |
| --------------------------------------------- | --------------------------------------------------------------------- |
| `KVCreateOrUpdate(ctx, config)`               | Create a KV bucket, or change it to agree with the configuration      |
| `KVGet(ctx, bucket, key)`                     | Read the entry of a key                                               |
| `KVPut(ctx, bucket, key, value)`              | Write a value to a key, and return the new revision                   |
| `KVCreate(ctx, bucket, key, value)`           | Write a value only when the key is not there                          |
| `KVUpdate(ctx, bucket, key, value, revision)` | Write a value only when the key is at that revision                   |
| `KVDelete(ctx, bucket, key)`                  | Delete a key                                                          |
| `KVKeys(ctx, bucket, filters...)`             | Return the keys that agree with a filter, or all keys with no filter  |
| `KVWatch(ctx, bucket, pattern, opts...)`      | Send each change to the keys that agree with the pattern on a channel |
| `KVStatus(ctx, bucket)`                       | Return the status of the bucket, for example its count of values      |

## Usage

```go
err := c.KVCreateOrUpdate(ctx, jetstream.KeyValueConfig{
    Bucket:   "job-queue",
    TTL:      1 * time.Hour,
    MaxBytes: 100 * 1024 * 1024,
    Storage:  jetstream.FileStorage,
})

revision, err := c.KVPut(ctx, "job-queue", "jobs.123", jobData)
entry, err := c.KVGet(ctx, "job-queue", "jobs.123")
keys, err := c.KVKeys(ctx, "job-queue", "jobs.>")
```

## Behavior to know

- A key that is not there makes `KVGet` return `jetstream.ErrKeyNotFound`.
- A bucket with no keys makes `KVKeys` return an empty list, not an error.
- When `ctx` ends during the list, `KVKeys` returns the error of `ctx`, not the
  keys that it got before.
- When the watch of `KVKeys` stops before the end of the keys, such as when the
  connection drops, `KVKeys` returns `ErrWatchStopped`, not a short list.
- `KVWatch` sends the current values first, then each change. NATS sends a nil
  entry after the current values. The channel does not send it. The channel
  closes when `ctx` ends, or when the watch stops. Thus a channel that closes
  while `ctx` is still live tells the caller that the watch ended.
- NATS cannot change the storage type of a bucket that exists. When only the
  storage type is different, `KVCreateOrUpdate` keeps the bucket as it is, and
  does not apply the other changes.

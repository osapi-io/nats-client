# Object store

These methods create Object Store buckets, and write, read and delete the
objects in them. A bucket keeps large blobs. NATS divides each blob into chunks.
Each method takes the name of the bucket. The client keeps a handle for each
bucket, as it does for a KV bucket.

## Methods

| Method                                    | Description                                                        |
| ----------------------------------------- | ------------------------------------------------------------------ |
| `ObjectStoreCreateOrUpdate(ctx, config)`  | Create a bucket, or change it to agree with the configuration      |
| `ObjectPut(ctx, bucket, meta, reader)`    | Write the object of `meta` from a reader                           |
| `ObjectPutBytes(ctx, bucket, name, data)` | Write bytes as an object                                           |
| `ObjectGetBytes(ctx, bucket, name)`       | Read the content of an object                                      |
| `ObjectInfo(ctx, bucket, name)`           | Read the information of an object, for example its size and digest |
| `ObjectList(ctx, bucket)`                 | Return the information of each object                              |
| `ObjectDelete(ctx, bucket, name)`         | Delete an object                                                   |
| `ObjectStoreStatus(ctx, bucket)`          | Return the status of the bucket, for example its size              |

## Usage

```go
err := c.ObjectStoreCreateOrUpdate(ctx, jetstream.ObjectStoreConfig{
    Bucket:   "file-objects",
    MaxBytes: 500 * 1024 * 1024,
    Storage:  jetstream.FileStorage,
})

info, err := c.ObjectPutBytes(ctx, "file-objects", "nginx.conf", content)
data, err := c.ObjectGetBytes(ctx, "file-objects", "nginx.conf")
err = c.ObjectDelete(ctx, "file-objects", "nginx.conf")
```

## Behavior to know

- An object that is not there makes `ObjectGetBytes` and `ObjectInfo` return
  `jetstream.ErrObjectNotFound`.
- A bucket with no objects makes `ObjectList` return an empty list, not an
  error. This agrees with `KVKeys`.
- A bucket with a different storage type stays as it is, as with
  `KVCreateOrUpdate`.

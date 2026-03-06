# Object Store

Manage NATS Object Store buckets for large blob storage with automatic
chunking.

## Methods

| Method                                          | Description                                       |
| ----------------------------------------------- | ------------------------------------------------- |
| `CreateOrUpdateObjectStore(ctx, config)`        | Create or update an Object Store bucket            |
| `ObjectStore(ctx, name)`                        | Get an existing Object Store handle                |

## Usage

```go
// Create an Object Store bucket
objStore, err := c.CreateOrUpdateObjectStore(ctx, jetstream.ObjectStoreConfig{
    Bucket:   "file-objects",
    MaxBytes: 500 * 1024 * 1024,
    Storage:  jetstream.FileStorage,
})

// Store a file
info, err := objStore.PutBytes(ctx, "nginx.conf", fileContent)

// Retrieve a file
data, err := objStore.GetBytes(ctx, "nginx.conf")

// Get metadata
info, err := objStore.GetInfo(ctx, "nginx.conf")

// Delete
err = objStore.Delete(ctx, "nginx.conf")
```

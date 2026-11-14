# Object store

Create and open NATS Object Store buckets. A bucket stores large blobs. NATS
divides each blob into chunks.

## Methods

| Method                                   | Description                              |
| ---------------------------------------- | ---------------------------------------- |
| `CreateOrUpdateObjectStore(ctx, config)` | Create or update an Object Store bucket  |
| `ObjectStore(ctx, name)`                 | Get a handle to an existing Object Store |

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

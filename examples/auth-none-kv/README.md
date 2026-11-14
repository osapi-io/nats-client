# NATS key-value store without authentication

This example creates a NATS Key-Value (KV) store and operates on it. It uses no
authentication.

## Usage

Run the client:

```bash
$ go run main.go
```

Get the KV bucket information:

```bats
$ nats kv info responses
```

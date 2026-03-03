# Connection Management

Connect to a NATS server with configurable authentication.

## Methods

| Method      | Description                                             |
| ----------- | ------------------------------------------------------- |
| `Connect()` | Establish connection to NATS and initialize JetStream   |
| `Close()`   | Close the underlying NATS connection                    |

## Usage

```go
// No authentication
c := client.New(logger, &client.Options{
    Host: "localhost",
    Port: 4222,
    Name: "my-app",
})

// Username/password authentication
c := client.New(logger, &client.Options{
    Host: "localhost",
    Port: 4222,
    Name: "my-app",
    Auth: client.AuthOptions{
        AuthType: client.UserPassAuth,
        Username: "osapi",
        Password: "secret",
    },
})

// NKey authentication
c := client.New(logger, &client.Options{
    Host: "localhost",
    Port: 4222,
    Name: "my-app",
    Auth: client.AuthOptions{
        AuthType: client.NKeyAuth,
        NKeyFile: "/path/to/seed.nk",
    },
})

if err := c.Connect(); err != nil {
    log.Fatal(err)
}
defer c.NC.Close()
```

# Connection management

The client connects to a NATS server. You can select the authentication mode.

## Methods

| Method      | Description                                             |
| ----------- | ------------------------------------------------------- |
| `Connect()` | Open the connection to NATS and make a JetStream handle |
| `Close()`   | Close the NATS connection                               |

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

// Credentials file authentication, for a server in operator (JWT) mode
c := client.New(logger, &client.Options{
    Host: "localhost",
    Port: 4222,
    Name: "my-app",
    Auth: client.AuthOptions{
        AuthType:  client.CredsAuth,
        CredsFile: "/path/to/user.creds",
    },
})

if err := c.Connect(); err != nil {
    log.Fatal(err)
}
defer c.Close()
```

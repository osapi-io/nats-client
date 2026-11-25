# Connection management

The client connects to a NATS server. You select the authentication type.

## Methods

| Method                     | Description                                                  |
| -------------------------- | ------------------------------------------------------------ |
| `Connect()`                | Open the connection to NATS and make a JetStream handle      |
| `Close()`                  | Close the NATS connection                                    |
| `ConnectedURL()`           | The URL of the connected server, or empty when not connected |
| `ConnectedServerVersion()` | The version of the connected server, or empty when not known |

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

// TLS, with any of the modes above: the roots to trust, and a client
// certificate when the server asks for one
c := client.New(logger, &client.Options{
    Host: "nats.example.com",
    Port: 4222,
    Name: "my-app",
    TLS:  &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
})

if err := c.Connect(); err != nil {
    log.Fatal(err)
}
defer c.Close()
```

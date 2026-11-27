# Connection management

The client connects to a NATS server. You select the authentication type.

## Methods

| Method                     | Description                                                  |
| -------------------------- | ------------------------------------------------------------ |
| `Connect()`                | Open the connection to NATS and make a JetStream handle      |
| `Close()`                  | Close the NATS connection, and forget the handles            |
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

## When the connection drops

The client tries to connect again, forever by default, `DefaultReconnectWait` (2
s) apart. `Options.Reconnect` changes this:

| Field               | Zero value                  | Does                                                  |
| ------------------- | --------------------------- | ----------------------------------------------------- |
| `MaxAttempts`       | try forever                 | The tries in a row, after which the connection closes |
| `Wait`              | `DefaultReconnectWait`, 2 s | The time between two tries                            |
| `RetryFirstConnect` | off                         | `Connect` succeeds with no server, and tries again    |

```go
// A process that runs for a long time, and can start before the server
c := client.New(logger, &client.Options{
    Host:      "nats.example.com",
    Port:      4222,
    Reconnect: client.ReconnectOptions{RetryFirstConnect: true},
})
```

The client logs each drop, each reconnect, the close of the connection and each
error that the server sends, such as a permissions violation, with its logger. A
credentials file is read again at each reconnect. Thus a renewed file is used at
the next reconnect.

Leave `RetryFirstConnect` off for a command that runs one time. Then `Connect`
returns the error of the first try, such as a wrong address or a refused
credential, and does not wait.

## Behavior to know

- A call before `Connect`, or after `Close`, returns `ErrNotConnected`. This
  includes `PublishCore` and `Subscribe`. `ConnectedURL` and
  `ConnectedServerVersion` return an empty string then.
- A call can run while another goroutine calls `Close` or `Connect`. It returns
  its result or `ErrNotConnected`.
- A second `Connect` without a `Close` between returns `ErrConnected`.
- `Connect` on a client that `New` made with nil options returns `ErrNoOptions`.
  A nil logger in `New` is `slog.Default()`.
- When JetStream fails to start, `Connect` closes the connection that it made,
  so that the connection does not keep reconnecting.
- `Close` forgets the handle of each bucket and stream. A bucket or a stream
  that the server says is gone, or a handle whose connection closed, is also
  forgotten, so the next call asks the server again. A call that finishes after
  a `Close` and the next `Connect` keeps no handle of the old connection.
- With an injected `KeyPair`, `Connect` does not read the seed file. It clears
  the seed bytes of the file after it parses them.
- An IPv6 address in `Host` connects, such as `::1`.

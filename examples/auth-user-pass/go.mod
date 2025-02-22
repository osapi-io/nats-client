module example.com/server

go 1.24

toolchain go1.24.0

replace github.com/osapi-io/nats-client => ../../../nats-client/

require (
	github.com/nats-io/nats.go v1.39.1
	github.com/osapi-io/nats-client v0.0.0-00010101000000-000000000000
)

require (
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/nats-io/nkeys v0.4.9 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)

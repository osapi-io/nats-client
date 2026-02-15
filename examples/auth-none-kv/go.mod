module example.com/server

go 1.25.0

replace github.com/osapi-io/nats-client => ../../../nats-client/

require github.com/osapi-io/nats-client v0.0.0-00010101000000-000000000000

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/nats-io/nats.go v1.48.0 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)

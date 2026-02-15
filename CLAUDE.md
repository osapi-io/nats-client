# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go client library for NATS JetStream providing connection management, JetStream contexts, KV stores, KV-backed streams, and consumer helpers. Used by osapi-io projects (linked via `replace` in consuming project's `go.mod`).

## Development Commands

```bash
just fetch             # Fetch shared justfiles (run once or to update)
just deps              # Install all dependencies (bats)
just test              # Run all tests (lint + unit + coverage + bats)
just go::unit          # Run unit tests only
just go::vet           # Run golangci-lint
just go::fmt           # Auto-format (gofumpt + golines)
just go::fmt-check     # Check formatting without modifying
just go::unit-cov      # Generate coverage report
go test -run TestName -v ./pkg/client/...  # Run a single test
```

## Package Structure

- **`pkg/client/`** - Core NATS client library
  - `client.go` - Client struct and constructor
  - `connect.go` / `connect_wrapper.go` - NATS connection management
  - `jetstream.go` - JetStream context helpers
  - `kv.go` - Key-value store operations
  - `kv_stream.go` - KV-backed stream operations
  - `consumer.go` - JetStream consumer helpers
  - `types.go` - Shared types and interfaces
  - `mocks/` - Generated mock implementations

## Code Standards (MANDATORY)

### Function Signatures

ALL function signatures MUST use multi-line format:
```go
func FunctionName(
    param1 type1,
    param2 type2,
) (returnType, error) {
}
```

### Testing

- Public tests: `*_public_test.go` in test package (`package client_test`) for exported functions
- Internal tests: `*_test.go` in same package (`package client`) for private functions
- Use `testify/suite` with table-driven patterns
- Use `golang/mock` for mocking interfaces

### Go Patterns

- Error wrapping: `fmt.Errorf("context: %w", err)`
- Early returns over nested if-else
- Unused parameters: rename to `_`
- Import order: stdlib, third-party, local (blank-line separated)

### Linting

golangci-lint with: errcheck, errname, goimports, govet, prealloc, predeclared, revive, staticcheck. Generated files (`*.gen.go`, `*.pb.go`) are excluded from formatting.

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/) with the
50/72 rule. Format: `type(scope): description`.

When committing via Claude Code, end with:
- `Co-Authored-By: Claude <noreply@anthropic.com>`

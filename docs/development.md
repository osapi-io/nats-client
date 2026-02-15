# Development

This guide covers the tools, setup, and conventions needed to work on
nats-client.

## Prerequisites

Install tools using [mise][]:

```bash
mise install
```

- **[Go][]** — nats-client is written in Go. We always support the latest two
  major Go versions, so make sure your version is recent enough.
- **[just][]** — Task runner used for building, testing, formatting, and other
  development workflows. Install with `brew install just`.

### Claude Code

If you use [Claude Code][] for development, install the **commit-commands**
plugin from the default marketplace:

```
/plugin install commit-commands@claude-plugins-official
```

This provides `/commit` and `/commit-push-pr` slash commands that follow the
project's commit conventions automatically.

## Setup

Fetch shared justfiles and install all dependencies:

```bash
just fetch
just deps
```

## Code style

Go code should be formatted by [`gofumpt`][gofumpt] and linted using
[`golangci-lint`][golangci-lint]. This style is enforced by CI.

```bash
just go::fmt-check   # Check formatting
just go::fmt         # Auto-fix formatting
just go::vet         # Run linter
```

## Testing

```bash
just test           # Run all tests (lint + unit + coverage + bats)
just go::unit       # Run unit tests only
just go::unit-cov   # Generate coverage report
go test -run TestName -v ./pkg/client/...  # Run a single test
```

### Test file conventions

- Public tests: `*_public_test.go` in test package (`package client_test`) for
  exported functions.
- Internal tests: `*_test.go` in same package (`package client`) for private
  functions.
- Use `testify/suite` with table-driven patterns.
- Use `golang/mock` for mocking interfaces.

## Commit messages

Follow [Conventional Commits][] with the 50/72 rule:

- **Subject line**: max 50 characters, imperative mood, capitalized, no period
- **Body**: wrap at 72 characters, separated from subject by a blank line
- **Format**: `type(scope): description`
- **Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`,
  `chore`
- Summarize the "what" and "why", not the "how"

Try to write meaningful commit messages and avoid having too many commits on a
PR. Most PRs should likely have a single commit (although for bigger PRs it may
be reasonable to split it in a few). Git squash and rebase is your friend!

[mise]: https://mise.jdx.dev
[Go]: https://go.dev
[just]: https://just.systems
[Claude Code]: https://claude.ai/code
[gofumpt]: https://github.com/mvdan/gofumpt
[golangci-lint]: https://golangci-lint.run
[Conventional Commits]: https://www.conventionalcommits.org

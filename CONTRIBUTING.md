# Contributing

Contributions to nats-client are very welcome, but we ask that you read this
document before submitting a PR. It covers everything you need: prerequisites,
setup, the conventions code follows, and the pull request workflow.

## Before you start

- Read the [Code of Conduct](CODE_OF_CONDUCT.md). It applies to every
  interaction in this repo.
- **Check existing work** — Is there an existing PR? Are there issues discussing
  the feature/change you want to make? Please make sure you consider/address
  these discussions in your work.
- **Backwards compatibility** — Will your change break existing consumers of
  nats-client? It is much more likely that your change will be merged if it is
  backwards compatible. Is there an approach you can take that maintains this
  compatibility? If not, consider opening an issue first so that API changes can
  be discussed before you invest your time into a PR.

## Prerequisites

Install tools using [mise]:

```bash
mise install
```

- **[Go]** — nats-client is written in Go. We always support the latest two
  major Go versions, so make sure your version is recent enough.
- **[uv]** — Python package runner. `just md-fmt` formats markdown with
  [mdformat] through `uvx`; nothing is installed into the repository.
- **[just]** — Task runner used for building, testing, formatting, and other
  development workflows. Install with `brew install just`.

### Claude Code

If you use [Claude Code] for development, install these plugins from the default
marketplace:

```
/plugin install commit-commands@claude-plugins-official
/plugin install superpowers@claude-plugins-official
```

- **commit-commands** — provides `/commit` and `/commit-push-pr` slash commands
  that follow the project's commit conventions automatically.
- **superpowers** — provides structured workflows for planning, TDD, debugging,
  code review, and git worktree isolation.

## Setup

Fetch shared justfiles and install all dependencies:

```bash
just fetch
just deps
```

## Code style

### Go

Go code should be formatted by [gofumpt] and linted using [golangci-lint]. This
style is enforced by CI.

```bash
just go-fmt-check   # Check formatting
just go-fmt         # Auto-fix formatting
just go-vet         # Run linter
```

golangci-lint runs errcheck, errname, goimports, govet, prealloc, predeclared,
revive, and staticcheck. Generated files (`*.gen.go`, `*.pb.go`) are excluded
from formatting.

### Function signatures

Functions with parameters MUST use multi-line format:

```go
func FunctionName(
    param1 type1,
    param2 type2,
) (returnType, error) {
}
```

### Go patterns

- Error wrapping: `fmt.Errorf("context: %w", err)`
- Early returns over nested if-else
- Unused parameters: rename to `_`
- Import order: stdlib, third-party, local (blank-line separated)
- Avoid generic file names like `helpers.go` or `utils.go` — name files after
  what they contain

### Documentation

Markdown files are formatted with [mdformat] through `uvx`. This style is
enforced by CI.

```bash
just md-fmt-check   # Check formatting
just md-fmt         # Auto-fix formatting
```

## Testing

```bash
just test           # Run all tests (lint + unit + coverage + bats)
just go-unit       # Run unit tests only
just go-unit-cov   # Generate coverage report
go test -run TestName -v ./pkg/client/...  # Run a single test
```

Coverage is gated at 100%. `just test` fails if total coverage drops below it,
so a change that adds untested code fails locally and in CI:

```bash
just go-unit-cov-check   # Report coverage and fail below the target
```

The target is declared in `.github/codecov.yml` and in the shared `go` justfile
module — change both together.

### Test file conventions

- Public tests: `*_public_test.go` in test package (`package client_test`) for
  exported functions.
- Internal tests: `*_test.go` in the same package (`package client`) for private
  functions.
- Suite naming: `*_public_test.go` → `{Name}PublicTestSuite`, `*_test.go` →
  `{Name}TestSuite`.
- Use `testify/suite` with table-driven patterns and `validateFunc` callbacks.
- **One suite method per function under test.** All scenarios for a function
  (success, error codes, transport failures, nil responses) belong as rows in a
  single table — never split into separate `TestFoo`, `TestFooError`,
  `TestFooNilResponse` methods.
- Use `golang/mock` for mocking interfaces.

## Before committing

Run `just ready` before committing to ensure generated code, package docs,
formatting, and lint are all up to date:

```bash
just ready
```

## Branching

All changes should be developed on feature branches. Create a branch from `main`
using the naming convention `type/short-description`, where `type` matches the
[Conventional Commits] type:

- `feat/add-retry-logic`
- `fix/null-pointer-crash`
- `docs/update-api-reference`
- `refactor/simplify-handler`
- `chore/update-dependencies`

When using Claude Code's `/commit` command, a branch will be created
automatically if you are on `main`.

## Commit messages

Follow [Conventional Commits] with the 50/72 rule:

- **Subject line**: max 50 characters, imperative mood, capitalized, no period
- **Body**: wrap at 72 characters, separated from subject by a blank line
- **Format**: `type(scope): description`
- **Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`
- Summarize the "what" and "why", not the "how"

Try to write meaningful commit messages and avoid having too many commits on a
PR. Most PRs should likely have a single commit (although for bigger PRs it may
be reasonable to split it in a few). Git squash and rebase is your friend!

## Submitting a PR

- **Describe your changes** — Ensure that you provide a comprehensive
  description of your changes.
- **Issue/PR links** — Link any previous work such as related issues or PRs.
  Please describe how your changes differ to/extend this work.
- **Examples** — Add any examples or screenshots that you think are useful to
  demonstrate the effect of your changes.
- **Draft PRs** — If your changes are incomplete, but you would like to discuss
  them, open the PR as a draft and add a comment to start a discussion. Using
  comments rather than the PR description allows the description to be updated
  later while preserving any discussions.

## AI usage

All contributions are subject to the [AI Usage Policy](AI_POLICY.md) — disclose
the tool you used, and make sure you can explain what your change does without
the aid of AI tools.

## FAQ

> I want to contribute, where do I start?

All kinds of contributions are welcome, whether it's a typo fix or a shiny new
feature. You can also contribute by upvoting/commenting on issues or helping to
answer questions.

> I'm stuck, where can I get help?

If you have questions, feel free to open a [Discussion] on GitHub.

[claude code]: https://claude.ai/code
[conventional commits]: https://www.conventionalcommits.org
[discussion]: https://github.com/osapi-io/nats-client/discussions
[go]: https://go.dev
[gofumpt]: https://github.com/mvdan/gofumpt
[golangci-lint]: https://golangci-lint.run
[just]: https://just.systems
[mdformat]: https://pypi.org/project/mdformat/
[mise]: https://mise.jdx.dev
[uv]: https://docs.astral.sh/uv/

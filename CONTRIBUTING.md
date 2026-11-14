# Contributing

We welcome contributions to nats-client. Read this document before you submit a
PR. It gives the prerequisites, the setup, the conventions for code, and the PR
workflow.

## Before you start

- Read the [Code of Conduct](CODE_OF_CONDUCT.md). It applies to all
  communication in this repository.
- **Design docs.** The design of this repository is in
  [osapi-io/specs](https://github.com/osapi-io/specs), under
  `components/nats-client/`. Read it before you change the behavior. When you
  change the behavior, change the design doc too. A design doc in this
  repository becomes incorrect when the code changes, and no check finds the
  difference.
- **Check existing work.** Look for an open PR or issue about the same change.
  If you find one, read its discussion and use it in your work.
- **Backwards compatibility.** Find out if your change breaks programs that
  import nats-client. We are more likely to merge a change that is backwards
  compatible. Look for a way to keep the compatibility. If there is no way, open
  an issue first. The issue lets us discuss the API change before you start work
  on a PR.

## Prerequisites

Install tools using [mise]:

```bash
mise install
```

- **[Go].** nats-client is written in Go. We support the latest two major Go
  versions. Make sure that your version is one of them.
- **[uv].** A Python package runner. `just md-fmt` formats markdown with
  [mdformat] through `uvx`. It does not install packages into the repository.
- **[just].** A task runner. It builds, tests, formats, and runs other
  development tasks. To install it, run `brew install just`.

### Claude Code

If you use [Claude Code], install this plugin from the default marketplace:

```
/plugin install commit-commands@claude-plugins-official
```

- **commit-commands.** It gives the `/commit` and `/commit-push-pr` slash
  commands. These commands follow the commit conventions of the project.

**The design is in the design docs.** Write or change the page in
[osapi-io/specs](https://github.com/osapi-io/specs) before you build. If the
build shows that the page is wrong, correct the page. A design doc in this
repository becomes incorrect when the code changes, and no check finds the
difference.

## Setup

Fetch the shared justfiles and install all dependencies:

```bash
just fetch
just deps
```

## Project structure

- **`pkg/client/`.** The public SDK. It is the only package that other programs
  import.
  - `types.go`. `Client`, `Options`, `AuthOptions`, `AuthType`. Type
    declarations only.
  - `client.go`. `New()`.
  - `connect.go`. `Connect()`, and the code that selects the auth type.
  - `connection.go`. Connection lifecycle and accessors: `Close()`,
    `ConnectedURL()`, `KeyValue()`, `Stream()`.
  - `connect_wrapper.go`. `NATSConnector`, an interface in front of
    `*nats.Conn`. Tests use it to replace the connection.
  - `core.go`. Core NATS: `Subscribe()`, `PublishCore()`.
  - `jetstream.go`. Stream and consumer management.
  - `consumer.go`. Message consumption, and calls to the handler.
  - `kv.go`, `kv_stream.go`. Key-value buckets, and request/reply over KV.
  - `objectstore.go`. Object store buckets.
  - `mocks/`. Generated mocks. See [Test doubles](#test-doubles).
- **`examples/`.** Programs that you can run, one for each auth mode and
  workload.

## Code style

[gofumpt] formats the Go code and [golangci-lint] examines it. CI makes these
checks mandatory.

```bash
just go-fmt-check   # Check formatting
just go-fmt         # Auto-fix formatting
just go-vet         # Run linter
```

`.golangci.yml` declares the linters that run. Read the list there, not here. A
copy of the list becomes incorrect when the configuration changes. The formatter
does not format generated files (`*.gen.go`, `*.pb.go`).

### Documentation

[mdformat] formats the markdown files through `uvx`. CI makes this format
mandatory.

```bash
just md-fmt-check   # Check formatting
just md-fmt         # Auto-fix formatting
```

## Code standards

### Function signatures

A function with parameters uses more than one line. Put each parameter on its
own line. Put the closing parenthesis and the return types on a line of their
own:

```go
func FunctionName(
    param1 type1,
    param2 type2,
) (returnType, error) {
}
```

A function with no parameters stays on one line:

```go
func Name() string {
}
```

When you add a parameter, the diff shows one new line. It does not show a new
signature.

### File naming

Give a file a name that tells what it holds. Do not use `helpers.go`,
`utils.go`, or names of that type. These names tell where the code is, not what
it is. Over time, they collect all code that has no other file.

`types.go` holds only type declarations: structs, interfaces, constants, and
aliases. Put a function in a file with a name that tells what the function does.

A test file has the name of the production file that it tests. If the tests
become too large to read, split the production file first. Then each test file
keeps one production file. Do not split the tests away from the file that they
test.

### Go patterns

- Error wrapping: `fmt.Errorf("context: %w", err)`. Then the chain names each
  layer that the error went through. `errors.Is` and `errors.As` can still
  examine the chain.
- Return early. Do not put the successful path inside conditionals.
- Unused parameters: change the name to `_`.
- Import order: standard library, third party, then local. Put a blank line
  between each group.

### Test doubles

Use `mockgen` to generate a double for an interface that this organization
defines, and commit the double. Do not write a struct by hand to satisfy the
interface.

Generated mocks are in a `mocks` package next to the code that they mock. A
`generate.go` file holds the directive that generates them:

```go
package mocks

//go:generate go tool go.uber.org/mock/mockgen -source=../types.go -destination=types.gen.go -package=mocks
```

Go finds the generator through the tool dependencies of the module. Thus each
checkout runs the version that `go.mod` records. The destination files end in
`.gen.go`, and you commit them. Do not use `gen/` for mocks. API code generation
uses that name.

If the interface is **unexported**, a sibling package does not work. The mock
must import the package to name the types in the interface. The tests of the
package must import the mock. Thus, generate the mock into the package. Use a
destination file that only tests compile. Then the mocking library is not in the
dependency graph of code that imports the package:

```go
// generate.go, in the package that declares the interface
package thispackage

//go:generate go tool go.uber.org/mock/mockgen -source=thing.go -destination=thing.gen_test.go -package=thispackage
```

In both cases, the directives are in a `generate.go` that holds no code. The
generated file has `.gen` in its name. This tells a reader not to edit it.

If many call sites set the same expectations, write a constructor that returns a
configured mock. Do not write a type by hand. The generated mock still satisfies
the interface.

Write these three doubles by hand, because a generated double gives no
advantage:

- A double for a standard library interface: `net.Conn`, `fs.File`, `io.Writer`,
  `slog.Handler`. These interfaces do not change when our code changes.
- A double that has a real implementation of the behavior under test. An example
  is a double that signs with a key pair that the test really generates.
- A recorder for a dependency that a goroutine calls, if the test cannot join
  that goroutine. A generated mock would assert a call count at a time that the
  test cannot control. Write this reason where you define the recorder.

### File headers

Each `.go` file MUST start with the MIT license header. For the exact format,
look at a Go file in the repository. A file with a build tag has `//go:build` on
line 1, then a blank line, then the header.

## Testing

```bash
just test           # Run all tests (lint + unit + coverage + bats)
just go-unit       # Run unit tests only
just go-unit-cov   # Generate coverage report
go test -run TestName -v ./pkg/client/...  # Run a single test
```

The coverage gate is 100%. `just test` fails if the total coverage is less than
100%. Thus a change that adds code with no tests fails locally and in CI:

```bash
just go-unit-cov-check   # Report coverage and fail below the target
```

`.github/codecov.yml` and the shared `go` justfile module both declare the
target. Change the two at the same time.

### Test file conventions

- Public tests: `*_public_test.go` in the external `_test` package. They test
  the exported API. Use this type of test first.
- Internal tests: `*_test.go` in the same package. They test the code that the
  exported API cannot reach.
- Suite naming: `*_public_test.go` → `{Name}PublicTestSuite`, `*_test.go` →
  `{Name}TestSuite`.
- `testify/suite` with table-driven cases.
- One suite method for each function under test. Success, errors, and edge cases
  are rows in one table. They are not different methods.
- `export_test.go` gives external tests access to unexported symbols, through an
  alias or a setter. Do not use an alias to test behavior again that the test of
  the caller already reaches. Use this pattern for a helper that has its own
  contract.

External tests in this repository are in `package client_test`. Their tables
have `validateFunc` callbacks.

## Before committing

Run `just ready` before you commit. It makes sure that the generated code, the
package docs, the format, and the lint are all current:

```bash
just ready
```

## Branching

Make all changes on feature branches. Create a branch from `main`. Use the name
format `type/short-description`. The `type` is the [Conventional Commits] type:

- `feat/add-retry-logic`
- `fix/null-pointer-crash`
- `docs/update-api-reference`
- `refactor/simplify-handler`
- `chore/update-dependencies`

If you are on `main`, the Claude Code `/commit` command creates a branch for
you.

## Commit messages

Follow [Conventional Commits] with the 50/72 rule:

- **Subject line**: 50 characters maximum, imperative mood, a capital letter
  first, no period
- **Body**: lines of 72 characters maximum, with a blank line between it and the
  subject
- **Format**: `type(scope): description`
- **Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`
- Give the "what" and the "why", not the "how"

Write commit messages that tell what changed. Do not put too many commits on a
PR. Usually a PR has one commit. A large PR can have a small number of commits.
Use git squash and git rebase to combine commits.

## Submitting a PR

- **Describe your changes.** Tell what changed and why. A reviewer must not have
  to read the diff to learn the reason.
- **Issue/PR links.** Link to related work, for example issues or PRs. Tell how
  your changes are different from that work, or how they add to it.
- **Examples.** Add examples or screenshots that show the effect of your
  changes.
- **Draft PRs.** If your changes are not complete and you want to discuss them,
  open the PR as a draft. Then add a comment to start the discussion. Use
  comments, not the PR description. Then you can change the description later,
  and the discussion stays.

## AI usage

We write this repository with help from AI. The [AI Usage Policy](AI_POLICY.md)
applies to all contributions. Tell which tool you used. Make sure that you can
explain your change without help from AI tools.

## FAQ

> I want to contribute, where do I start?

We welcome all types of contributions, for example a typo fix or a new feature.
You can also vote on issues, comment on issues, or help to answer questions.

> I'm stuck, where can I get help?

If you have questions, open a [Discussion] on GitHub.

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

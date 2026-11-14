# AGENTS.md

Test: `just test` | Before committing: `just ready`

Read [CONTRIBUTING.md](CONTRIBUTING.md) first. It gives the prerequisites, the
setup, the code style, the test conventions, and the PR workflow. All of these
rules apply to agents the same as to people. This file has only the rules that
are specific to agents.

## Running tools

Run tools through `mise`, not from your path:

```bash
mise exec -- just test
```

`mise` is active in the shell of a person. It supplies the versions that
`.mise.toml` declares. The shell of an agent does not activate `mise`. Thus a
bare `just` runs the version that is installed globally, which is usually older.

The symptom is a check that fails here and passes in continuous integration, on
a file that nobody changed. If this occurs, find which version ran. Do this
before you accept the failure as real.

## Where the rules come from

[CONTRIBUTING.md](CONTRIBUTING.md#before-you-start) names the specification. If
a convention here and the specification do not agree, obey the specification.
Say that they do not agree. Do not follow the code.

## Commit trailer

When you commit with Claude Code, end the message with this text:

```
🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

## Task tracking

**The design is in the design documents.** Write or change the page in
[osapi-io/specs](https://github.com/osapi-io/specs) before you build. If the
build shows that the page is wrong, correct the page. A design document in this
repository becomes incorrect when the code changes, and no check finds the
difference.

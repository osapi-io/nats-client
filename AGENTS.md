# AGENTS.md

Test: `just test` | Before committing: `just ready`

Read @CONTRIBUTING.md first. It covers prerequisites, setup, code style, test
conventions, and the pull request workflow — all of which apply to agents
exactly as they apply to people. This file carries only what is specific to
agents.

## Where the rules come from

Repository layout and shared tooling are specified in
[osapi-io/specs](https://github.com/osapi-io/specs). When a convention here and
the specification disagree, the specification wins — say so rather than
following the code.

## Commit trailer

When committing via Claude Code, end the message with:

```
🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

## Task tracking

Implementation planning and execution uses the superpowers plugin workflows
(`writing-plans` and `executing-plans`). Plans live in `docs/plans/`.

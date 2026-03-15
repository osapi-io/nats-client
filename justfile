# Optional modules: mod? allows `just fetch` to work before .just/remote/ exists.
# Recipes below use `just` subcommands instead of dependency syntax because just
# validates dependencies at parse time, which would fail when modules aren't loaded.
mod? go '.just/remote/go.mod.just'
mod? just '.just/remote/just.mod.just'

# --- Fetch ---

# Fetch shared justfiles from osapi-justfiles
fetch:
    mkdir -p .just/remote
    curl -sSfL https://raw.githubusercontent.com/osapi-io/osapi-justfiles/refs/heads/main/go.mod.just -o .just/remote/go.mod.just
    curl -sSfL https://raw.githubusercontent.com/osapi-io/osapi-justfiles/refs/heads/main/go.just -o .just/remote/go.just
    curl -sSfL https://raw.githubusercontent.com/osapi-io/osapi-justfiles/refs/heads/main/just.mod.just -o .just/remote/just.mod.just
    curl -sSfL https://raw.githubusercontent.com/osapi-io/osapi-justfiles/refs/heads/main/just.just -o .just/remote/just.just

# --- Top-level orchestration ---

# Install all dependencies
deps:
    just go::deps
    go get -tool github.com/golang/mock/mockgen

# Run all tests
test:
    just just::fmt-check
    just go::test

# Format and lint before committing
ready:
    just go::fmt
    just go::vet

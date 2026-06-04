# Contributing Guide

## Development Environment

- Go 1.25 or later
- `make`
- `git`

Clone the repository and install the helper tools. This installs the linter,
the coverage tool, and `shellspec` (used by the end-to-end tests):

```bash
make tools
```

`make tools` installs shellspec under `~/.local`, so make sure `~/.local/bin` is
on your `PATH`.

## Common Commands

```bash
make build      # build the iso8583tool binary
make test       # unit tests with coverage (writes coverage.out / coverage.html)
make test-e2e   # shellspec end-to-end tests against the built binary
make lint       # golangci-lint
```

The end-to-end tests live under `spec/` and exercise the built binary the way a
user does (subcommands, flags, stdin, exit codes) using the fixtures in
`examples/basei`. Run them with `make test-e2e`, or directly with
`shellspec --shell sh` after `make build`.

## Pull Request Expectations

- keep CLI behavior and error messages consistent
- add or update tests for new behavior, including a `spec/` test for CLI changes
- run `make test` and `make test-e2e` before opening a PR
- run `make lint` when changing Go code
- update `CHANGELOG.md` under `[Unreleased]`

## CI

GitHub Actions runs the following workflows, and every gate is reproducible
locally with the `make` targets above:

- `build.yml`: verifies the project builds on Linux
- `unit_test.yml`: runs `go test ./...` on Linux, macOS, and Windows (`make test`)
- `e2e_test.yml`: runs the shellspec end-to-end tests on Linux and macOS (`make test-e2e`)
- `coverage.yml`: reports coverage with octocov
- `reviewdog.yml`: comments on lint, misspell, and workflow issues in pull requests

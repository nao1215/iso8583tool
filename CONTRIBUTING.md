# Contributing Guide

## Development Environment

- Go 1.25 or later
- `make`
- `git`

Clone the repository and install the helper tools. This installs the linter,
the coverage tool, and `atago` (used by the end-to-end tests):

```bash
make tools
```

## Common Commands

```bash
make build      # build the iso8583tool binary
make test       # unit tests with coverage (writes coverage.out / coverage.html)
make e2e        # atago end-to-end tests against a freshly built binary
make lint       # golangci-lint
```

The end-to-end tests live under `e2e/atago/` as plain-YAML
[atago](https://github.com/nao1215/atago) specs and exercise the built binary
the way a user does (subcommands, flags, stdin, exit codes) using the fixtures
in `examples/basei`. The `send` scenarios drive the TCP client against the
single-shot mock server in `e2e/mock`. Run them with `make e2e`, or directly
with `e2e/run.sh` (which also accepts atago flags, e.g. `e2e/run.sh --filter
send`).

## Pull Request Expectations

- keep CLI behavior and error messages consistent
- add or update tests for new behavior, including an `e2e/atago/` spec for CLI changes
- run `make test` and `make e2e` before opening a PR
- run `make lint` when changing Go code
- update `CHANGELOG.md` under `[Unreleased]`

## CI

GitHub Actions runs the following workflows, and every gate is reproducible
locally with the `make` targets above:

- `build.yml`: verifies the project builds on Linux
- `unit_test.yml`: runs `go test ./...` on Linux, macOS, and Windows (`make test`)
- `e2e_test.yml`: runs the atago end-to-end tests on Linux and macOS (`make e2e`)
- `coverage.yml`: reports coverage with octocov
- `reviewdog.yml`: comments on lint, misspell, and workflow issues in pull requests
- `release.yml`: publishes tagged release artifacts with GoReleaser

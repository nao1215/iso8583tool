#!/bin/sh
# shellcheck shell=sh
#
# shellspec helper for iso8583tool end-to-end tests. These drive the built
# binary the way a user does (subcommands, flags, stdin, exit codes, files on
# disk) so they catch regressions the Go tests cannot. Read-only tests use the
# fixtures under examples/; tests that write files use a throwaway mktemp dir.

set -eu

PROJECT_ROOT="$(cd "$SHELLSPEC_SPECDIR/.." && pwd)"
export PROJECT_ROOT

# ISO_BIN points at the binary built by `make build`. Override to test another
# build.
ISO_BIN="${ISO_BIN:-$PROJECT_ROOT/iso8583tool}"
export ISO_BIN

# EXAMPLES is the bundled BASE I fixture directory.
EXAMPLES="$PROJECT_ROOT/examples/basei"
export EXAMPLES

# iso8583tool runs the built binary.
iso8583tool() {
  "$ISO_BIN" "$@"
}

# sample_hex prints a packed sample message as hex.
sample_hex() {
  "$ISO_BIN" sample 0110-auth-response --format hex
}

make_workdir() {
  WORK="$(mktemp -d)"
  export WORK
}

remove_workdir() {
  if [ -n "${WORK:-}" ]; then
    rm -rf "$WORK"
  fi
}

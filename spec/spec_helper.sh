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

PACKED_BCD_SPEC="spec87bcd-starter"
export PACKED_BCD_SPEC

# iso8583tool runs the built binary.
iso8583tool() {
  "$ISO_BIN" "$@"
}

# sample_hex prints a packed sample message as hex.
sample_hex() {
  "$ISO_BIN" sample 0110-auth-response --format hex
}

# write_kanmu_like_message writes a raw-binary ISO8583 message that uses a
# packed-BCD MTI, a binary bitmap, a one-byte PAN length, and packed-BCD
# numeric fields. It mirrors the layout used in kanmu/gocon-2022-spring.
write_kanmu_like_message() {
  printf '\001\000\160\004\000\000\000\000\000\000\020\100\031\044\231\231\231\231\231\062\163\047\000\000\000\000\021\070\042\004' > "$1"
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

# --- send mock server helpers -------------------------------------------------
#
# The send command needs a live TCP peer. spec/mock is a deterministic,
# single-shot, 127.0.0.1-only server that reuses the production framing code: it
# reads one framed request and replies with a fixed 0810 response framed the same
# way. These helpers build it once and start a fresh instance per example, so
# both send_spec.sh and readme_spec.sh drive the real wire path without an
# external network and without flaking.

# build_mock compiles the mock once. go build can write progress/cache notices to
# stderr (which ShellSpec treats as a failing hook), so its output is captured
# and only surfaced on a real error.
build_mock() {
  MOCK_DIR="$(mktemp -d)"
  MOCK_BIN="$MOCK_DIR/mock"
  if ! ( cd "$PROJECT_ROOT" && go build -o "$MOCK_BIN" ./spec/mock ) >"$MOCK_DIR/build.log" 2>&1; then
    echo "failed to build the send mock server:" >&2
    cat "$MOCK_DIR/build.log" >&2
    return 1
  fi
  REPLY_HEX="$(tr -d ' \t\n\r' < "$EXAMPLES/0810-network-echo-response.hex")"
  export MOCK_DIR MOCK_BIN REPLY_HEX
}

remove_mock() { [ -n "${MOCK_DIR:-}" ] && rm -rf "$MOCK_DIR"; }

# start_mock FRAMING [extra-args...] launches a fresh single-shot mock and waits
# until it has published its listen address into the ready file. Requires a
# per-example WORK dir (call make_workdir first).
start_mock() {
  framing="$1"
  shift
  READY="$WORK/ready"
  rm -f "$READY"
  "$MOCK_BIN" --framing "$framing" --reply-hex "$REPLY_HEX" --ready-file "$READY" "$@" >"$WORK/mock.log" 2>&1 &
  MOCK_PID=$!
  i=0
  while [ ! -s "$READY" ] && [ "$i" -lt 100 ]; do
    # If the mock died before publishing its address, fail now with its log
    # instead of reading a stale/empty file and connecting to a bad address.
    if ! kill -0 "$MOCK_PID" 2>/dev/null; then
      echo "mock server exited before it was ready:" >&2
      cat "$WORK/mock.log" >&2
      return 1
    fi
    sleep 0.05
    i=$((i + 1))
  done
  if [ ! -s "$READY" ]; then
    echo "mock server did not publish its address within the timeout:" >&2
    cat "$WORK/mock.log" >&2
    return 1
  fi
  MOCK_ADDR="$(cat "$READY")"
}

stop_mock() {
  if [ -n "${MOCK_PID:-}" ]; then
    kill "$MOCK_PID" 2>/dev/null || true
    wait "$MOCK_PID" 2>/dev/null || true
    MOCK_PID=""
  fi
}

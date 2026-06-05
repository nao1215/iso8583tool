#!/bin/sh
# shellcheck shell=sh
#
# send: drive the TCP client against a local, in-repo mock server. The mock
# (spec/mock) is built once, listens on 127.0.0.1 with an ephemeral port, reads
# one framed request, and replies with a fixed 0810 response framed the same
# way. No external network is used; the response is deterministic so the tests
# do not flake.

Describe 'iso8583tool send'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  # build_mock compiles the mock server once for the whole file. go build can
  # write progress or cache notices to stderr (which ShellSpec would treat as a
  # failing hook), so its output is captured and only surfaced on a real error.
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

  BeforeAll 'build_mock'
  AfterAll 'remove_mock'

  # start_mock FRAMING [extra-args...] launches a fresh single-shot mock and
  # waits until it has published its listen address into the ready file.
  start_mock() {
    framing="$1"
    shift
    READY="$WORK/ready"
    rm -f "$READY"
    "$MOCK_BIN" --framing "$framing" --reply-hex "$REPLY_HEX" --ready-file "$READY" "$@" >/dev/null 2>&1 &
    MOCK_PID=$!
    i=0
    while [ ! -s "$READY" ] && [ "$i" -lt 100 ]; do
      sleep 0.05
      i=$((i + 1))
    done
    MOCK_ADDR="$(cat "$READY")"
  }
  stop_mock() {
    if [ -n "${MOCK_PID:-}" ]; then
      kill "$MOCK_PID" 2>/dev/null || true
      wait "$MOCK_PID" 2>/dev/null || true
      MOCK_PID=""
    fi
  }

  Describe '2byte-binary framing'
    BeforeEach 'make_workdir; start_mock 2byte-binary'
    AfterEach 'stop_mock; remove_workdir'

    It 'sends an 0800 and decodes the 0810 response'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --framing 2byte-binary
      The status should be success
      The output should include 'Framing:'
      The output should include '2byte-binary'
      The output should include 'Request:'
      The output should include 'Response:'
      The output should include '0810'
    End

    It 'packs a JSON document and sends it'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.json" --framing 2byte-binary --format json
      The status should be success
      The output should include '"framing": "2byte-binary"'
      The output should include '"remote_addr"'
      The output should include '"rtt_ms"'
      The output should include '"sent_bytes"'
      The output should include '"received_bytes"'
      The output should include '"request_view"'
      The output should include '"response_view"'
      The output should include '"mti": "0810"'
    End

    It 'reads the message from stdin via -'
      When run sh -c '"$ISO_BIN" send "$1" - --framing 2byte-binary < "$2"' sh "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex"
      The status should be success
      The output should include '0810'
      The output should include 'Response:'
    End
  End

  Describe '4digit-ascii framing'
    BeforeEach 'make_workdir; start_mock 4digit-ascii'
    AfterEach 'stop_mock; remove_workdir'

    It 'frames with a 4-digit ASCII length header'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --framing 4digit-ascii --format json
      The status should be success
      The output should include '"framing": "4digit-ascii"'
      The output should include '"mti": "0810"'
    End
  End

  Describe 'timeout'
    BeforeEach 'make_workdir; start_mock 2byte-binary --no-reply'
    AfterEach 'stop_mock; remove_workdir'

    It 'exits non-zero with a clear error when the response times out'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --framing 2byte-binary --timeout 600ms
      The status should be failure
      The stderr should include 'timed out'
    End
  End

  Describe 'invalid arguments'
    It 'rejects an invalid --framing value'
      When run iso8583tool send 127.0.0.1:1 "$EXAMPLES/0800-network-echo.hex" --framing bogus
      The status should be failure
      The stderr should include 'invalid --framing'
    End
  End

  Describe 'help'
    It 'prints usage for send --help'
      When run iso8583tool send --help
      The status should be success
      The output should include 'Usage: iso8583tool send'
      The output should include '--framing'
    End
  End
End

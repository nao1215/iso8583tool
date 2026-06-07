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

  # The mock build/start/stop helpers live in spec_helper.sh so the README spec
  # can drive the same in-repo server.
  BeforeAll 'build_mock'
  AfterAll 'remove_mock'

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

    It 'lists every response field, not only annotated codes'
      # F41/F48/F63 carry no decoded meaning but must still be visible for a
      # fault investigation, consistent with view.
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --framing 2byte-binary --no-color
      The status should be success
      The output should include '41 = TERMNET1'
      The output should include '48 = HEARTBEAT=BASEI'
      The output should include '63 = ECHO=OK'
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

  Describe 'none framing'
    BeforeEach 'make_workdir; start_mock none'
    AfterEach 'stop_mock; remove_workdir'

    It 'sends with no length header and reads the reply until EOF'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --framing none --format json
      The status should be success
      The output should include '"framing": "none"'
      The output should include '"mti": "0810"'
    End

    It 'decodes the response in describe output'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --framing none
      The status should be success
      The output should include 'none'
      The output should include 'Response:'
      The output should include '0810'
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

  Describe 'none framing timeout'
    BeforeEach 'make_workdir; start_mock none --no-reply'
    AfterEach 'stop_mock; remove_workdir'

    It 'exits non-zero when a none-framing peer never replies'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --framing none --timeout 600ms
      The status should be failure
      The stderr should include 'timed out'
    End
  End

  Describe 'expectations'
    BeforeEach 'make_workdir; start_mock 2byte-binary'
    AfterEach 'stop_mock; remove_workdir'

    It 'passes when --expect-mti and --expect-field match the response'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --expect-mti 0810 --expect-field 39=00 --expect-field 70=301
      The status should be success
      The output should include '0810'
    End

    It 'exits non-zero with a deterministic error on an MTI mismatch'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --expect-mti 0800
      The status should be failure
      # The exchange is still printed (so a failing run shows the response).
      The output should include 'Response:'
      The stderr should include 'send expectation failed:'
      The stderr should include 'MTI: expected "0800", got "0810"'
    End

    It 'exits non-zero when an expected field value differs'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --expect-field 39=99
      The status should be failure
      The output should include 'Response:'
      The stderr should include 'send expectation failed:'
      The stderr should include 'F39: expected "99", got "00"'
    End

    It 'rejects an --expect-field without PATH=VALUE'
      When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --expect-field 39
      The status should be failure
      The stderr should include 'invalid --expect-field'
    End
  End

  Describe 'dry run'
    # --dry-run never opens a connection, so these need no mock server. Port 1 has
    # nothing listening; a live send would fail to connect, but --dry-run succeeds.
    It 'frames and prints the request without connecting'
      When run iso8583tool send 127.0.0.1:1 "$EXAMPLES/0800-network-echo.hex" --dry-run
      The status should be success
      The output should include 'Dry run'
      The output should include 'Would send bytes:'
      The output should include 'Request:'
      The output should include '0800'
    End

    It 'emits a machine-readable dry-run record'
      When run iso8583tool send 127.0.0.1:1 "$EXAMPLES/0800-network-echo.hex" --dry-run --format json
      The status should be success
      The output should include '"dry_run": true'
      The output should include '"would_send_bytes"'
      The output should include '"mti": "0800"'
    End

    It 'withholds the framed bytes by default'
      When run iso8583tool send 127.0.0.1:1 "$EXAMPLES/0800-network-echo.hex" --dry-run --no-color
      The status should be success
      The output should not include 'Framed bytes'
    End

    It 'reveals the framed wire bytes under --unsafe'
      When run iso8583tool send 127.0.0.1:1 "$EXAMPLES/0800-network-echo.hex" --dry-run --unsafe --no-color
      The status should be success
      The output should include 'Framed bytes:'
    End

    It 'includes framed_hex in JSON only under --unsafe'
      When run iso8583tool send 127.0.0.1:1 "$EXAMPLES/0800-network-echo.hex" --dry-run --unsafe --format json
      The status should be success
      The output should include '"framed_hex"'
    End

    It 'rejects expectations because there is no response to assert'
      When run iso8583tool send 127.0.0.1:1 "$EXAMPLES/0800-network-echo.hex" --dry-run --expect-mti 0810
      The status should be failure
      The stderr should include 'dry-run'
    End
  End

  Describe 'invalid arguments'
    It 'rejects an invalid --framing value'
      When run iso8583tool send 127.0.0.1:1 "$EXAMPLES/0800-network-echo.hex" --framing bogus
      The status should be failure
      The stderr should include 'invalid --framing'
    End

    It 'rejects a HOST:PORT without a port'
      When run iso8583tool send 127.0.0.1 "$EXAMPLES/0800-network-echo.hex" --timeout 500ms
      The status should be failure
      The stderr should include 'invalid address'
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

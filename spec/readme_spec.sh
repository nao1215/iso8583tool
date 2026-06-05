#!/bin/sh
# shellcheck shell=sh
#
# README command snippets. Keep these aligned with README.md so the documented
# examples are exercised in CI and stay copy-pasteable.

Describe 'README examples'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'quick start'
    It 'lists the bundled samples'
      When run iso8583tool sample
      The status should be success
      The output should include '0100-auth-request'
    End

    It 'views the BASE I auth response'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex"
      The status should be success
      The output should include 'Summary:'
    End

    It 'validates the unknown-TLV sample'
      When run iso8583tool validate "$EXAMPLES/0100-auth-request-unknown-tlv.hex"
      The status should be success
      The output should include '55.DF8129'
    End

    It 'converts the BASE I request to JSON'
      When run iso8583tool convert "$EXAMPLES/0100-auth-request.hex"
      The status should be success
      The output should include '"mti": "0100"'
    End
  End

  Describe 'view'
    It 'shows JSON output'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --format json
      The status should be success
      The output should include '"fields"'
    End

    It 'filters the requested fields'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --filter 39 --filter 55.8A
      The status should be success
      The output should include 'Approved'
    End

    It 'reads a message from stdin'
      When run sh -c 'cat "$EXAMPLES/0110-auth-response.hex" | "$ISO_BIN" view -'
      The status should be success
      The output should include 'MTI'
    End

    It 'is jq-compatible for fields'
      When run sh -c '"$ISO_BIN" view "$EXAMPLES/0110-auth-response.hex" --format json | jq -r ".fields[\"39\"]"'
      The status should be success
      The output should equal '00'
    End
  End

  Describe 'diff'
    It 'compares a request and a response'
      When run iso8583tool diff "$EXAMPLES/0100-auth-request.hex" "$EXAMPLES/0110-auth-response.hex"
      The status should be success
      The output should include 'changed'
    End

    It 'is jq-compatible for changes'
      When run sh -c '"$ISO_BIN" diff "$EXAMPLES/0100-auth-request.hex" "$EXAMPLES/0110-auth-response.hex" --format json | jq -r ".changes[].path" | head -n1'
      The status should be success
      The output should be present
    End
  End

  Describe 'redact'
    It 'masks the PAN for safe sharing'
      When run iso8583tool redact "$EXAMPLES/0100-auth-request.hex"
      The status should be success
      The output should not include '4111111111111111'
    End

    It 'supports a text format'
      When run iso8583tool redact "$EXAMPLES/0100-auth-request.hex" --format text
      The status should be success
      The output should include 'Redacted:'
    End
  End

  Describe 'convert'
    It 'packs the BASE I request to hex'
      When run iso8583tool convert "$EXAMPLES/0100-auth-request.json"
      The status should be success
      The output should match pattern '3031*'
    End

    It 'converts a sample through stdin'
      When run sh -c '"$ISO_BIN" sample 0100-auth-request --format hex | "$ISO_BIN" convert'
      The status should be success
      The output should include '"mti": "0100"'
    End

    It 'writes converted output to a file'
      When run sh -c 'tmp="$(mktemp)"; "$ISO_BIN" convert "$EXAMPLES/0100-auth-request.json" --output "$tmp" && test -s "$tmp"'
      The status should be success
      The output should include 'Converted with'
    End
  End

  Describe 'validate'
    It 'reports a broken inline message as an error'
      When run iso8583tool validate --raw 01007220
      The status should be failure
      The output should include '[error]'
    End

    It 'emits JSON when asked'
      When run iso8583tool validate "$EXAMPLES/0110-auth-response.hex" --format json
      The status should be success
      The output should include '"valid": true'
    End
  End

  Describe 'doctor'
    It 'recommends a preset for the BASE I sample'
      When run iso8583tool doctor "$EXAMPLES/0110-auth-response.hex"
      The status should be success
      The output should include 'Recommended: --spec basei-starter'
    End

    It 'is jq-compatible for the recommendation'
      When run sh -c '"$ISO_BIN" doctor "$EXAMPLES/0110-auth-response.hex" --format json | jq -r .recommended'
      The status should be success
      The output should equal 'basei-starter'
    End
  End

  Describe 'specs'
    It 'lists the presets'
      When run iso8583tool specs
      The status should be success
      The output should include 'basei-starter (default)'
    End

    It 'is jq-compatible for preset names'
      When run sh -c '"$ISO_BIN" specs --format json | jq -r ".[].name" | head -n1'
      The status should be success
      The output should equal 'basei-starter'
    End
  End

  Describe 'sample'
    It 'prints a sample as JSON'
      When run iso8583tool sample 0100-auth-request
      The status should be success
      The output should include '"mti": "0100"'
    End

    It 'writes a sample as hex'
      When run sh -c 'tmp="$(mktemp)"; "$ISO_BIN" sample 0100-auth-request --format hex --output "$tmp" && test -s "$tmp"'
      The status should be success
      The output should include 'Wrote sample'
    End
  End

  Describe 'send'
    # Drive the README send snippets against the in-repo, 127.0.0.1-only mock
    # (built once here) so the documented examples stay executable in CI.
    BeforeAll 'build_mock'
    AfterAll 'remove_mock'

    Describe 'default 2byte-binary framing'
      BeforeEach 'make_workdir; start_mock 2byte-binary'
      AfterEach 'stop_mock; remove_workdir'

      It 'sends a packed 0800 and decodes the 0810 reply'
        When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex"
        The status should be success
        The output should include '0810'
      End

      It 'reads the message from stdin and is jq-compatible for the response MTI'
        When run sh -c '"$ISO_BIN" sample 0800-network-echo --format hex | "$ISO_BIN" send "$1" - --format json | jq -r ".response_view.mti"' sh "$MOCK_ADDR"
        The status should be success
        The output should equal '0810'
      End

      It 'asserts the reply with --expect-mti / --expect-field (no jq needed)'
        When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0800-network-echo.hex" --expect-mti 0810 --expect-field 39=00
        The status should be success
        The output should include '0810'
      End

      It 'accepts an inline message via --raw'
        When run iso8583tool send "$MOCK_ADDR" --raw '{"mti":"0800","fields":{"70":"301","11":"654321","41":"TERMNET1"}}' --format json
        The status should be success
        The output should include '"mti": "0810"'
      End
    End

    Describe '4-digit ASCII framing'
      BeforeEach 'make_workdir; start_mock 4digit-ascii'
      AfterEach 'stop_mock; remove_workdir'

      It 'packs a JSON document and sends it with a 4-digit header'
        When run iso8583tool send "$MOCK_ADDR" "$EXAMPLES/0100-auth-request.json" --framing 4digit-ascii --format json
        The status should be success
        The output should include '"framing": "4digit-ascii"'
        The output should include '"mti": "0810"'
      End
    End
  End

  Describe 'unknown TLV round-trip'
    It 'preserves the unknown tag when unpacking and packing again'
      When run sh -c '"$ISO_BIN" convert "$EXAMPLES/0100-auth-request-unknown-tlv.hex" | "$ISO_BIN" convert | "$ISO_BIN" view - --filter 55.DF8129'
      The status should be success
      The output should include 'DF8129'
    End
  End

  Describe 'other specs'
    It 'validates the spec87ascii sample'
      When run iso8583tool validate "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --spec spec87ascii
      The status should be success
      The output should include 'Spec: spec87ascii'
    End

    It 'strict-validates the spec87ascii sample under its intended preset'
      When run iso8583tool validate "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --spec spec87ascii --strict
      The status should be success
      The output should include 'ok'
    End

    It 'views the spec87ascii sample'
      When run iso8583tool view "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --spec spec87ascii
      The status should be success
      The output should include '0800'
    End

    It 'converts the spec87ascii sample to JSON'
      When run iso8583tool convert "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --spec spec87ascii
      The status should be success
      The output should include '"mti": "0800"'
    End
  End
End

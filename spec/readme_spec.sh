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
      The output should include '"message"'
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

  Describe 'unknown TLV round-trip'
    It 'preserves the unknown tag when unpacking and packing again'
      When run sh -c '"$ISO_BIN" convert "$EXAMPLES/0100-auth-request-unknown-tlv.hex" | "$ISO_BIN" convert | "$ISO_BIN" view - --filter 55.DF8129'
      The status should be success
      The output should include 'DF8129'
    End
  End

  Describe 'other specs'
    It 'validates the spec87ascii sample'
      When run iso8583tool validate "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --config "$PROJECT_ROOT/examples/spec87ascii.config.json"
      The status should be success
      The output should include 'Spec: spec87ascii'
    End

    It 'views the spec87ascii sample'
      When run iso8583tool view "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --config "$PROJECT_ROOT/examples/spec87ascii.config.json"
      The status should be success
      The output should include '0800'
    End

    It 'converts the spec87ascii sample to JSON'
      When run iso8583tool convert "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --config "$PROJECT_ROOT/examples/spec87ascii.config.json"
      The status should be success
      The output should include '"mti": "0800"'
    End
  End
End

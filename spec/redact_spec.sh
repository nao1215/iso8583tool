#!/bin/sh
# shellcheck shell=sh
#
# redact: deterministic masking of cardholder data and secrets for safe sharing,
# JSON (default) and text output, jq compatibility, and stdin.

Describe 'iso8583tool redact'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'masks the PAN in JSON output'
    When run sh -c '"$ISO_BIN" redact "$EXAMPLES/0100-auth-request.hex" | jq -r ".fields[\"2\"]"'
    The status should be success
    The output should equal '411111******1111'
  End

  It 'never leaks the full PAN'
    When run iso8583tool redact "$EXAMPLES/0100-auth-request.hex"
    The status should be success
    The output should not include '4111111111111111'
  End

  It 'fully masks the EMV application cryptogram'
    When run sh -c '"$ISO_BIN" redact "$EXAMPLES/0100-auth-request.hex" | jq -r ".binary_fields[\"55.9F26\"]"'
    The status should be success
    The output should equal '****************'
  End

  It 'supports a human-readable text format'
    When run iso8583tool redact "$EXAMPLES/0100-auth-request.hex" --format text
    The status should be success
    The output should include 'Redacted:'
    The output should include '411111******1111'
  End

  It 'reads from stdin for a Slack-safe pipe'
    When run sh -c 'cat "$EXAMPLES/0100-auth-request.hex" | "$ISO_BIN" redact -'
    The status should be success
    The output should not include '4111111111111111'
  End
End

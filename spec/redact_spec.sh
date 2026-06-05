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

  It 'orders text output by MTI then numeric field id'
    When run iso8583tool redact "$EXAMPLES/0100-auth-request.hex" --format text --color never
    The status should be success
    The line 1 of output should include 'MTI:'
    The line 2 of output should include 'F2 ='
  End

  It 'reads from stdin for a Slack-safe pipe'
    When run sh -c 'cat "$EXAMPLES/0100-auth-request.hex" | "$ISO_BIN" redact -'
    The status should be success
    The output should not include '4111111111111111'
  End

  It 'masks a PAN embedded in a free-form private field (F63)'
    When run sh -c 'printf "%s" "{\"mti\":\"0110\",\"fields\":{\"11\":\"123456\",\"39\":\"00\",\"63\":\"PAN=4111111111111111\"}}" | "$ISO_BIN" convert --to hex | "$ISO_BIN" redact -'
    The status should be success
    The output should not include '4111111111111111'
  End

  Describe 'auto-detected input encoding'
    BeforeEach 'make_workdir; write_kanmu_like_message "$WORK/message.bin"'
    AfterEach 'remove_workdir'

    # redact must default to --encoding auto like the other commands, so a raw
    # *.bin capture is read without an explicit --encoding raw.
    It 'redacts a raw binary capture without --encoding'
      When run iso8583tool redact "$WORK/message.bin" --spec "$PACKED_BCD_SPEC"
      The status should be success
      The output should include '"mti"'
    End

    It 'still masks the PAN in a raw binary capture'
      When run sh -c '"$ISO_BIN" redact "$1" --spec "$2" | jq -r ".fields[\"2\"]"' sh "$WORK/message.bin" "$PACKED_BCD_SPEC"
      The status should be success
      The output should include '*'
    End
  End
End

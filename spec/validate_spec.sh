#!/bin/sh
# shellcheck shell=sh
#
# validate: exit codes (0 for warnings, 1 for errors), the field that broke, and
# the unknown-TLV warning.

Describe 'iso8583tool validate'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'passes a good message with exit 0'
    When run iso8583tool validate "$EXAMPLES/0110-auth-response.hex"
    The status should be success
    The output should include 'Validation: ok'
    The output should include 'MTI: 0110'
  End

  It 'reports unknown TLV tags as a warning but still exits 0'
    When run iso8583tool validate "$EXAMPLES/0100-auth-request-unknown-tlv.hex"
    The status should be success
    The output should include 'warning'
    The output should include '55.DF8129'
  End

  It 'fails a broken message with exit 1 and names the field'
    When run iso8583tool validate --raw 01007220
    The status should be failure
    The output should include 'Validation: failed'
    The output should include '[error]'
    The output should include 'input was'
  End

  It 'emits a JSON report with --format json'
    When run iso8583tool validate "$EXAMPLES/0110-auth-response.hex" --format json
    The status should be success
    The output should include '"valid": true'
    The output should include '"summary"'
  End

  It 'accepts a complete sample under --strict'
    When run iso8583tool validate "$EXAMPLES/0110-auth-response.hex" --strict
    The status should be success
    The output should include 'Validation: ok'
  End

  It 'flags a hollow response under --strict'
    # A 0110 carrying only a STAN unpacks, but is not a well-formed response.
    When run sh -c 'printf "%s" "{\"mti\":\"0110\",\"fields\":{\"11\":\"123456\"}}" | "$ISO_BIN" convert --to hex | "$ISO_BIN" validate - --strict'
    The status should be failure
    The output should include 'Validation: failed'
    The output should include '39'
  End
End

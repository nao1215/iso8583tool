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
End

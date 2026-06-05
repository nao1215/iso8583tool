#!/bin/sh
# shellcheck shell=sh
#
# canonical field values: the full describe view, the filtered view, and the
# decoded[] entries must all show a zero-padded fixed-length field with the same
# canonical width (for example F3 "000000", F4 "000000005000"), not the
# collapsed integer form field.String() returns.

Describe 'iso8583tool canonical field values'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  MSG="$EXAMPLES/0110-auth-response.hex"

  It 'shows F3 with canonical width in the full describe view' # bug 11
    When run iso8583tool view "$MSG" --no-color
    The status should be success
    The line 1 should be present
    The output should include 'Processing Code'
    # The F3 line carries the canonical, zero-padded processing code.
    The output should match pattern '*F3*: 000000*'
  End

  It 'matches the filtered view for F4' # bug 11
    When run iso8583tool view "$MSG" --filter 4 --no-color
    The status should be success
    The output should include '000000005000'
  End

  It 'returns canonical decoded values in JSON for F3' # bug 21
    When run iso8583tool view "$MSG" --format json
    The status should be success
    # Both the fields map and the decoded entry use the canonical width.
    The output should include '"3": "000000"'
    The output should include '"value": "000000"'
  End

  It 'returns canonical decoded values from validate for F3' # bug 21
    When run iso8583tool validate "$MSG" --format json
    The status should be success
    The output should include '"value": "000000"'
  End
End

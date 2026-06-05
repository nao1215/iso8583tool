#!/bin/sh
# shellcheck shell=sh
#
# filter ergonomics: view and diff must normalize the case of hex EMV tag paths,
# accept both "0" and "mti" for the MTI, and tell the user when a diff filter
# matched nothing (so a typo is distinguishable from a real no-change result).

Describe 'iso8583tool filter normalization'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  REQ="$EXAMPLES/0100-auth-request.hex"
  RESP="$EXAMPLES/0110-auth-response.hex"

  It 'matches a lowercase EMV tag in view' # bug 14
    When run iso8583tool view "$REQ" --filter 55.9f02 --no-color
    The status should be success
    The output should include '55.9F02'
    The output should not include '<not present>'
  End

  It 'matches a lowercase EMV tag in diff' # bug 14
    When run iso8583tool diff "$REQ" "$RESP" --filter 55.8a
    The status should be success
    The output should include '55.8A'
    The output should not include 'No field matched'
  End

  It 'accepts "0" as an MTI alias in diff' # bug 45
    When run iso8583tool diff "$REQ" "$RESP" --filter 0
    The status should be success
    The output should include 'MTI changed'
  End

  It 'reports an unmatched diff filter' # bug 46
    When run iso8583tool diff "$REQ" "$RESP" --filter 999
    The status should be success
    The output should include 'No field matched filter: 999'
  End

  It 'reports an unmatched filter in JSON' # bug 46
    When run iso8583tool diff "$REQ" "$RESP" --filter 999 --format json
    The status should be success
    The output should include '"missing_filters"'
    The output should include '999'
  End
End

#!/bin/sh
# shellcheck shell=sh
#
# diff: field-level comparison, JSON output (jq), filtering, stdin, and the
# identical-message case.

Describe 'iso8583tool diff'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  BeforeEach 'make_workdir'
  AfterEach 'remove_workdir'

  # setup_pair writes before.hex and an after.hex with field 4 and 55.9F02 bumped.
  setup_pair() {
    cp "$EXAMPLES/0100-auth-request.hex" "$WORK/before.hex"
    "$ISO_BIN" convert "$EXAMPLES/0100-auth-request.hex" \
      | sed 's/"000000005000"/"000000009999"/' \
      | "$ISO_BIN" convert > "$WORK/after.hex"
  }

  It 'reports changed fields in text form'
    setup_pair
    When run iso8583tool diff "$WORK/before.hex" "$WORK/after.hex"
    The status should be success
    The output should include 'Field 4 changed'
    The output should include '- 000000005000'
    The output should include '+ 000000009999'
  End

  It 'emits jq-compatible JSON'
    setup_pair
    When run sh -c '"$ISO_BIN" diff "$WORK/before.hex" "$WORK/after.hex" --format json | jq -r ".changes[0].kind"'
    The status should be success
    The output should equal 'changed'
  End

  It 'reports no differences for identical messages'
    When run iso8583tool diff "$EXAMPLES/0110-auth-response.hex" "$EXAMPLES/0110-auth-response.hex"
    The status should be success
    The output should include 'No differences.'
  End

  It 'filters to a field subtree'
    setup_pair
    When run iso8583tool diff "$WORK/before.hex" "$WORK/after.hex" --filter 55
    The status should be success
    The output should include '55.9F02'
    The output should not include 'Field 4 '
  End

  It 'reads one side from stdin'
    setup_pair
    When run sh -c 'cat "$WORK/after.hex" | "$ISO_BIN" diff "$WORK/before.hex" -'
    The status should be success
    The output should include 'changed'
  End

  It 'rejects two stdin sides'
    When run iso8583tool diff - -
    The status should be failure
    The stderr should include 'stdin'
  End
End

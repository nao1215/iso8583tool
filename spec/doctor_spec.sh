#!/bin/sh
# shellcheck shell=sh
#
# doctor: detect which built-in preset fits a message, across ASCII and packed
# BCD, and the no-fit / validate-hint paths.

Describe 'iso8583tool doctor'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'recommends the BASE I starter for an ASCII BASE I message'
    When run iso8583tool doctor "$EXAMPLES/0110-auth-response.hex"
    The status should be success
    The output should include 'Recommended: --spec basei-starter'
    The output should include 'Confirm with: iso8583tool view'
  End

  It 'detects a packed-BCD raw message'
    make_workdir
    write_kanmu_like_message "$WORK/message.bin"
    When run iso8583tool doctor "$WORK/message.bin" --encoding raw
    The status should be success
    The output should include "Recommended: --spec $PACKED_BCD_SPEC"
    remove_workdir
  End

  It 'auto-detects a raw .bin without --encoding'
    make_workdir
    write_kanmu_like_message "$WORK/message.bin"
    When run iso8583tool doctor "$WORK/message.bin"
    The status should be success
    The output should include '(raw input)'
    The output should include "Recommended: --spec $PACKED_BCD_SPEC"
    remove_workdir
  End

  It 'emits a JSON report with --format json'
    When run iso8583tool doctor "$EXAMPLES/0110-auth-response.hex" --format json
    The status should be success
    The output should include '"recommended": "basei-starter"'
    The output should include '"exact_round_trip": true'
  End

  It 'exits non-zero when no preset fits'
    When run iso8583tool doctor --raw fffefd
    The status should be failure
    The output should include 'No built-in preset could unpack'
  End

  It 'is suggested by a wrong-spec validate failure'
    # A complete message that fails at a data field under the wrong spec is the
    # case the doctor hint is for; a header-level failure is reported as
    # truncated/corrupt instead.
    When run iso8583tool validate "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --spec spec87bcd-starter
    The status should be failure
    The output should include 'doctor'
  End
End

#!/bin/sh
# shellcheck shell=sh
#
# input auto-detection: view, validate, convert (and diff/redact) default to
# --encoding auto, so a raw *.bin capture or an all-numeric raw ASCII message is
# read correctly without an explicit --encoding raw, the same way doctor does.

Describe 'iso8583tool input auto-detection'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'views a raw binary capture without --encoding raw'
    write_kanmu_like_message "$WORK/raw.bin"
    When run iso8583tool view "$WORK/raw.bin" --spec "$PACKED_BCD_SPEC"
    The status should be success
    The output should not include 'decode hex'
    The output should include 'MTI'
  End

  It 'validates a raw binary capture without --encoding raw'
    write_kanmu_like_message "$WORK/raw.bin"
    When run iso8583tool validate "$WORK/raw.bin" --spec "$PACKED_BCD_SPEC"
    The status should be success
    The output should not include 'decode hex'
  End

  It 'converts a raw binary capture without --encoding raw'
    write_kanmu_like_message "$WORK/raw.bin"
    When run iso8583tool convert "$WORK/raw.bin" --spec "$PACKED_BCD_SPEC"
    The status should be success
    The output should not include 'decode hex'
    The output should include '"mti"'
  End

  It 'reads an all-numeric raw ASCII capture as raw, not packed hex'
    printf '%s' '0800022000000000000000000000000000000604161616654321' > "$WORK/num.bin"
    When run iso8583tool view "$WORK/num.bin"
    The status should be success
    The output should not include 'not enough data'
    The output should include '0800'
  End
End

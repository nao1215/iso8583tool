#!/bin/sh
# shellcheck shell=sh
#
# Adversarial input: malformed hex, truncated messages, empty input, missing
# files, a directory where a file is expected, ambiguous input selection, and
# mismatched convert directions. These try to break the binary, and every one
# must fail cleanly (non-zero status, a message, no panic) rather than crash.

Describe 'iso8583tool edge cases'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'rejects non-hex characters'
    When run iso8583tool view --raw 'zzzz'
    The status should be failure
    The stderr should include 'hex'
  End

  It 'rejects odd-length hex'
    When run iso8583tool view --raw '0100712'
    The status should be failure
    The stderr should be present
  End

  It 'reports the failing field for a truncated message'
    When run iso8583tool validate --raw 01007220
    The status should be failure
    The output should include '[error]'
  End

  It 'fails on empty inline input'
    When run iso8583tool view --raw ''
    The status should be failure
    The stderr should be present
  End

  It 'fails when the file does not exist'
    When run iso8583tool view /no/such/message.hex
    The status should be failure
    The stderr should be present
  End

  It 'fails when a directory is passed instead of a file'
    When run iso8583tool view "$EXAMPLES"
    The status should be failure
    The stderr should be present
  End

  It 'refuses both a file argument and --raw'
    When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --raw 0100
    The status should be failure
    The stderr should be present
  End

  It 'fails to pack a document with no mti'
    When run sh -c 'printf "%s" "{\"fields\":{}}" | "$ISO_BIN" convert --to hex'
    The status should be failure
    The stderr should include 'mti'
  End

  It 'fails to pack a document with an invalid TLV tag'
    When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"binary_fields\":{\"55.ZZ\":\"00\"}}" | "$ISO_BIN" convert --to hex'
    The status should be failure
    The stderr should be present
  End
End

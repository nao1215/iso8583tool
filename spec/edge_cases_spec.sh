#!/bin/sh
# shellcheck shell=sh
#
# Adversarial input: malformed hex, truncated messages, empty input, missing
# files, a directory where a file is expected, ambiguous input selection, and
# mismatched convert directions. These try to break the binary, and every one
# must fail cleanly (non-zero status, a message, no panic) rather than crash.

Describe 'iso8583tool edge cases'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'rejects non-hex characters under --encoding hex'
    # The default --encoding auto reads non-hex input as raw; an explicit hex
    # encoding still reports the bad input as a hex error.
    When run iso8583tool view --encoding hex --raw 'zzzz'
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

  Describe 'oversized and length-spoofed input'
    BeforeEach 'make_workdir'
    AfterEach 'remove_workdir'

    It 'rejects an oversized file with a clear limit error'
      # ~1.05 MiB of '0' (valid, even-length hex) just over the 1 MiB source cap.
      head -c 1100000 /dev/zero | tr '\0' '0' > "$WORK/big.hex"
      When run iso8583tool view "$WORK/big.hex"
      The status should be failure
      The stderr should include 'limit'
    End

    It 'rejects oversized stdin with a clear limit error'
      When run sh -c 'head -c 1100000 /dev/zero | tr "\0" "0" | "$ISO_BIN" view -'
      The status should be failure
      The stderr should include 'limit'
    End

    It 'cleanly fails a truncated field 55 TLV without panic'
      # MTI + bitmap selecting F55 + a BER-TLV length that overruns the body.
      When run iso8583tool validate --raw 010000000000000008000103DF
      The status should be failure
      The output should include '[error]'
      The stderr should not include 'panic'
    End

    It 'does not panic on a length-spoofed variable-length field'
      # MTI + bitmap selecting F2 (LLVAR) with a length prefix larger than the data.
      When run iso8583tool view --raw 0100400000000000000099
      The status should be failure
      The stderr should not include 'panic'
    End
  End
End

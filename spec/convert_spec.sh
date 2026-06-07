#!/bin/sh
# shellcheck shell=sh
#
# convert: direction auto-detection, --to override, round-trip stability,
# stdin/pipes, and writing to a file.

Describe 'iso8583tool convert'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'auto-detected direction'
    It 'packs a JSON document to hex'
      When run iso8583tool convert "$EXAMPLES/0100-auth-request.json"
      The status should be success
      The output should match pattern '3031*'
    End

    It 'unpacks a message to a JSON document'
      When run iso8583tool convert "$EXAMPLES/0100-auth-request.hex"
      The status should be success
      The output should include '"mti": "0100"'
      The output should include '"55.9F02"'
    End
  End

  Describe '--to override'
    It 'forces json output from a message'
      When run iso8583tool convert "$EXAMPLES/0110-auth-response.hex" --to json
      The status should be success
      The output should include '"mti"'
    End

    It 'rejects an unknown direction'
      When run iso8583tool convert "$EXAMPLES/0100-auth-request.json" --to sideways
      The status should be failure
      The stderr should include 'unsupported --to'
    End
  End

  Describe 'document path conflicts'
    It 'rejects a path present in both fields and binary_fields'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"fields\":{\"55.8A\":\"00\"},\"binary_fields\":{\"55.8A\":\"3035\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include '55.8A'
    End

    It 'rejects a parent path that also has nested children'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"binary_fields\":{\"55\":\"9F0206000000005000\",\"55.9F02\":\"000000009999\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include '55.9F02'
    End

    It 'rejects field id 0 (reserved for the MTI)'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"fields\":{\"0\":\"9999\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'mti'
    End

    It 'rejects field id 1 (the bitmap)'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"fields\":{\"1\":\"1234\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'bitmap'
    End

    It 'rejects field id 0 set through binary_fields'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"binary_fields\":{\"0\":\"31323334\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'mti'
    End

    It 'rejects an out-of-range field id'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"fields\":{\"129\":\"x\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'invalid field id'
    End

    It 'rejects a non-numeric field id'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"fields\":{\"A.1\":\"x\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'invalid field id'
    End

    It 'rejects a malformed dotted path'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"binary_fields\":{\"55..9F02\":\"00\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'empty segment'
    End

    It 'rejects leading whitespace in a path key'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"fields\":{\" 2\":\"4111111111111111\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'whitespace'
    End

    It 'rejects a leading-zero duplicate alias (02 vs 2)'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"fields\":{\"02\":\"4111111111111111\",\"2\":\"4222222222222222\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'address field'
    End

    It 'rejects a case-different duplicate TLV alias (9f02 vs 9F02)'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"binary_fields\":{\"55.9f02\":\"000000001000\",\"55.9F02\":\"000000005000\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'address field'
    End

    It 'rejects raw bytes routed to a text field via binary_fields'
      When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"binary_fields\":{\"11\":\"000102030405\"}}" | "$ISO_BIN" convert --to hex'
      The status should be failure
      The stderr should include 'text field'
    End
  End

  Describe 'round-trip'
    It 'is stable through hex -> json -> hex'
      When run sh -c '
        h="$("$ISO_BIN" sample 0100-auth-request --format hex)"
        back="$(printf "%s" "$h" | "$ISO_BIN" convert | "$ISO_BIN" convert)"
        [ "$(printf "%s" "$h")" = "$(printf "%s" "$back")" ] && echo SAME || echo DIFF'
      The status should be success
      The output should equal 'SAME'
    End
  End

  Describe 'raw binary + packed BCD'
    BeforeEach 'make_workdir; write_kanmu_like_message "$WORK/message.bin"'
    AfterEach 'remove_workdir'

    It 'is stable through raw -> json -> raw with the packed-BCD starter preset'
      When run sh -c '
        "$ISO_BIN" convert "$1" --encoding raw --spec "$2" > "$3/doc.json" &&
        "$ISO_BIN" convert "$3/doc.json" --to hex --encoding raw --spec "$2" --output "$3/back.bin" >/dev/null &&
        cmp -s "$1" "$3/back.bin" &&
        echo SAME' sh "$WORK/message.bin" "$PACKED_BCD_SPEC" "$WORK"
      The status should be success
      The output should equal 'SAME'
    End
  End

  Describe 'to a file'
    BeforeEach 'make_workdir'
    AfterEach 'remove_workdir'

    It 'writes the result and reports it'
      When run iso8583tool convert "$EXAMPLES/0100-auth-request.json" --output "$WORK/out.hex"
      The status should be success
      The output should include 'Converted with'
      The output should include 'unmasked'
      The output should include 'sensitive'
      The path "$WORK/out.hex" should be file
    End
  End

  Describe 'unmasked-output warning'
    It 'documents the unmasked output in help'
      When run iso8583tool convert --help
      The status should be success
      The output should include 'UNMASKED'
      The output should include 'redact'
    End

    It 'stays byte-clean on stderr when piped (stdout not a TTY)'
      When run iso8583tool convert "$EXAMPLES/0100-auth-request.hex"
      The status should be success
      The output should include '"mti": "0100"'
      The stderr should equal ''
    End
  End
End

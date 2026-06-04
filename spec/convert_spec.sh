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

  Describe 'to a file'
    BeforeEach 'make_workdir'
    AfterEach 'remove_workdir'

    It 'writes the result and reports it'
      When run iso8583tool convert "$EXAMPLES/0100-auth-request.json" --output "$WORK/out.hex"
      The status should be success
      The output should include 'Converted with'
      The path "$WORK/out.hex" should be file
    End
  End
End

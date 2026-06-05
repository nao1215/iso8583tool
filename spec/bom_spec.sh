#!/bin/sh
# shellcheck shell=sh
#
# convert must tolerate a UTF-8 BOM (EF BB BF) at the start of a JSON document,
# both when auto-detecting the direction and when the direction is forced with
# --to hex. Editors and some exporters prepend the BOM; it is not valid JSON.

Describe 'iso8583tool convert with a UTF-8 BOM'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    make_workdir
    printf '\357\273\277%s' '{"mti":"0100","fields":{"11":"123456"}}' > "$WORK/bom.json"
  }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'auto-detects BOM-prefixed JSON as a document to pack'
    When run iso8583tool convert "$WORK/bom.json"
    The status should be success
    The output should not include 'invalid byte'
    The output should not include 'decode hex'
  End

  It 'packs BOM-prefixed JSON with an explicit --to hex'
    When run iso8583tool convert "$WORK/bom.json" --to hex
    The status should be success
    The output should not include 'invalid character'
  End

  Describe 'a BOM-prefixed hex fixture'
    bom_hex_setup() {
      make_workdir
      { printf '\357\273\277'; cat "$EXAMPLES/0110-auth-response.hex"; } > "$WORK/bom.hex"
    }
    BeforeEach 'bom_hex_setup'
    AfterEach 'cleanup'

    It 'views a BOM-prefixed hex file'
      When run iso8583tool view "$WORK/bom.hex" --no-color
      The status should be success
      The output should include '0110'
    End

    It 'doctors a BOM-prefixed hex file as hex, not raw'
      When run iso8583tool doctor "$WORK/bom.hex" --no-color
      The status should be success
      The output should include 'hex input'
    End

    It 'validates a BOM-prefixed hex file'
      When run iso8583tool validate "$WORK/bom.hex" --no-color
      The status should be success
      The output should include 'Validation: ok'
    End

    It 'converts a BOM-prefixed hex file to JSON'
      When run iso8583tool convert "$WORK/bom.hex" --to json
      The status should be success
      The output should include '"mti"'
    End
  End
End

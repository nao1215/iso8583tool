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
End

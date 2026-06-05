#!/bin/sh
# shellcheck shell=sh
#
# Standard secondary-bitmap fields: the built-in preset must define the common
# high-numbered ISO 8583 fields (95, 96, 100, 102-104) and the reserved range
# 123-128, so a representative document packs and round-trips instead of failing
# with "field N is not defined in the spec".

Describe 'iso8583tool standard high-numbered fields'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'packs and round-trips fields 95/96/100/102/103/104'
    printf '%s' '{"mti":"0200","fields":{"11":"123456","95":"000000000000000000000000000000000000000000","100":"12345678901","102":"1234567890123456789012345678","103":"8765432109876543210987654321","104":"DESCRIPTION"},"binary_fields":{"96":"A1B2C3D4E5F60708"}}' > "$WORK/hi.json"
    iso8583tool convert "$WORK/hi.json" --to hex > "$WORK/hi.hex"
    When run iso8583tool view "$WORK/hi.hex" --format json
    The status should be success
    The output should include '"100": "12345678901"'
    The output should include '"104": "DESCRIPTION"'
  End

  It 'packs the reserved fields 123-127'
    printf '%s' '{"mti":"0100","fields":{"11":"123456","123":"AAA","124":"BBB","125":"CCC","126":"DDD","127":"EEE"}}' > "$WORK/r.json"
    When run iso8583tool convert "$WORK/r.json" --to hex
    The status should be success
    The output should not include 'not defined'
  End

  It 'packs and round-trips the binary MAC field 128'
    printf '%s' '{"mti":"0100","binary_fields":{"128":"A1B2C3D4E5F60708"}}' > "$WORK/m.json"
    iso8583tool convert "$WORK/m.json" --to hex > "$WORK/m.hex"
    When run iso8583tool view "$WORK/m.hex" --unsafe --format json
    The status should be success
    The output should include '"128": "A1B2C3D4E5F60708"'
  End
End

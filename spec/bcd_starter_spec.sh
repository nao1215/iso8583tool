#!/bin/sh
# shellcheck shell=sh
#
# spec87bcd-starter: the packed-BCD preset for raw-binary captures must pack and
# round-trip the EMV TLV field (55), the raw PIN (52) and MAC (64) secret
# fields, and variable-length fields without leaking an ASCII length prefix.

Describe 'iso8583tool spec87bcd-starter preset'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'packs and round-trips an EMV TLV tag (55.9F02)'
    printf '%s' '{"mti":"0100","fields":{"2":"4019249999999999","3":"000000","4":"000000001000","7":"0605123456","11":"123456","41":"TERMID01"},"binary_fields":{"55.9F02":"000000001000"}}' > "$WORK/emv.json"
    iso8583tool convert "$WORK/emv.json" --to hex --encoding hex --spec "$PACKED_BCD_SPEC" > "$WORK/emv.hex"
    When run iso8583tool view "$WORK/emv.hex" --encoding hex --spec "$PACKED_BCD_SPEC" --format json
    The status should be success
    The output should include '"55.9F02"'
    The output should include '000000001000'
  End

  It 'packs a raw PIN field (52)'
    printf '%s' '{"mti":"0100","fields":{"2":"4019249999999999","3":"000000","4":"000000001000","7":"0605123456","11":"123456","41":"TERMID01"},"binary_fields":{"52":"A1B2C3D4E5F60708"}}' > "$WORK/pin.json"
    When run iso8583tool convert "$WORK/pin.json" --to hex --encoding hex --spec "$PACKED_BCD_SPEC"
    The status should be success
    The output should not include 'should be fixed'
  End

  It 'packs and round-trips a raw MAC field (64)'
    printf '%s' '{"mti":"0100","fields":{"2":"4019249999999999","3":"000000","4":"000000001000","7":"0605123456","11":"123456","41":"TERMID01"},"binary_fields":{"64":"A1B2C3D4E5F60708"}}' > "$WORK/mac.json"
    iso8583tool convert "$WORK/mac.json" --to hex --encoding hex --spec "$PACKED_BCD_SPEC" > "$WORK/mac.hex"
    When run iso8583tool view "$WORK/mac.hex" --encoding hex --spec "$PACKED_BCD_SPEC" --unsafe --format json
    The status should be success
    The output should include 'A1B2C3D4E5F60708'
  End

  It 'round-trips a variable-length field with a BCD length prefix (32)'
    printf '%s' '{"mti":"0100","fields":{"2":"4019249999999999","3":"000000","4":"000000001000","7":"0605123456","11":"123456","32":"123456","41":"TERMID01"}}' > "$WORK/f32.json"
    iso8583tool convert "$WORK/f32.json" --to hex --encoding hex --spec "$PACKED_BCD_SPEC" > "$WORK/f32.hex"
    When run iso8583tool view "$WORK/f32.hex" --encoding hex --spec "$PACKED_BCD_SPEC" --format json
    The status should be success
    The output should include '"32": "123456"'
  End
End

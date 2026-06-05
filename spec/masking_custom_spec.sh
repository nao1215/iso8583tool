#!/bin/sh
# shellcheck shell=sh
#
# Masking under custom specs: a PAN/track carried in a binary field, in a TLV
# tag that the spec makes "known", in a non-55 TLV container, or nested inside a
# constructed TLV template must still be masked by view and redact.

Describe 'iso8583tool masking under custom specs'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  # build packs a doc with the given spec and prints the hex path.
  build() { # $1 spec-json $2 doc-json
    printf '%s' "$1" > "$WORK/spec.json"
    printf '%s' "$2" > "$WORK/doc.json"
    iso8583tool convert "$WORK/doc.json" --to hex --spec "$WORK/spec.json" --output "$WORK/m.hex" >/dev/null
  }

  It 'masks a PAN in a binary field 63'
    build '{"name":"B63","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"63":{"type":"Binary","length":999,"description":"Private","enc":"Binary","prefix":"ASCII.LLL"}}}' \
      '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"63":"50414E3D34313131313131313131313131313131"}}'
    When run iso8583tool view "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '50414E3D'
  End

  build_9f6b() {
    build '{"name":"9F6B","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"55":{"type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL","tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"9F6B":{"type":"Binary","length":19,"description":"Track 2 Equivalent","enc":"Binary","prefix":"BerTLV"}}}}}' \
      '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"55.9F6B":"4111111111111111D29122011234567890"}}'
  }

  It 'masks a known 9F6B track2-equivalent tag in view'
    build_9f6b
    When run iso8583tool view "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '4111111111111111'
  End

  It 'masks a known 9F6B track2-equivalent tag in redact'
    build_9f6b
    When run iso8583tool redact "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '4111111111111111'
  End

  It 'masks a track2-equivalent tag in a non-55 container (127.57)'
    build '{"name":"T127","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"127":{"type":"Composite","length":999,"description":"Private TLV","prefix":"ASCII.LLL","tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"57":{"type":"Binary","length":18,"description":"Track2Eq","enc":"Binary","prefix":"BerTLV"}}}}}' \
      '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"127.57":"4111111111111111D29122011234567890"}}'
    When run iso8583tool view "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '4111111111111111'
  End

  It 'masks a sensitive tag nested in a constructed TLV (55.70.57)'
    build '{"name":"C57","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"55":{"type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL","tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"70":{"type":"Composite","length":255,"description":"Template","prefix":"BerTLV","tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"57":{"type":"Binary","length":18,"description":"Track2Eq","enc":"Binary","prefix":"BerTLV"}}}}}}}' \
      '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"55.70.57":"4111111111111111D29122011234567890"}}'
    When run iso8583tool redact "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '4111111111111111'
  End

  # A custom spec gives a field id its own meaning, so the BASE I positional rules
  # (field 35 = track, 52 = PIN) must not over-mask a harmless value. Content
  # scanning still masks anything PAN- or track-shaped, so nothing sensitive leaks.
  It 'does not over-mask a harmless custom field 35'
    build '{"name":"F35","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"35":{"type":"String","length":37,"description":"Partner Reference","enc":"ASCII","prefix":"ASCII.LL"}}}' \
      '{"mti":"0110","fields":{"11":"123456","35":"REF-ORDER-ABC-0001"}}'
    When run iso8583tool view "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should include 'REF-ORDER-ABC-0001'
  End

  It 'does not over-mask a harmless custom field 52'
    build '{"name":"F52","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"52":{"type":"String","length":8,"description":"Partner Status","enc":"ASCII","prefix":"ASCII.Fixed"}}}' \
      '{"mti":"0110","fields":{"11":"123456","52":"ABCDEFGH"}}'
    When run iso8583tool view "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should include 'ABCDEFGH'
  End

  It 'still masks a real PAN in a custom field 2'
    build '{"name":"F2","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"2":{"type":"String","length":19,"description":"Account","enc":"ASCII","prefix":"ASCII.LL"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"}}}' \
      '{"mti":"0110","fields":{"2":"4111111111111111","11":"123456"}}'
    When run iso8583tool view "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '4111111111111111'
  End
End

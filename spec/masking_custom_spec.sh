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

  It 'masks a PAN in a binary field 63' # bug 19
    build '{"name":"B63","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"63":{"type":"Binary","length":999,"description":"Private","enc":"Binary","prefix":"ASCII.LLL"}}}' \
      '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"63":"50414E3D34313131313131313131313131313131"}}'
    When run iso8583tool view "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '50414E3D'
  End

  It 'masks a known 9F6B track2-equivalent tag in view and redact' # bugs 31, 32
    build '{"name":"9F6B","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"55":{"type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL","tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"9F6B":{"type":"Binary","length":19,"description":"Track 2 Equivalent","enc":"Binary","prefix":"BerTLV"}}}}}' \
      '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"55.9F6B":"4111111111111111D29122011234567890"}}'
    When run iso8583tool redact "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '4111111111111111'
  End

  It 'masks a track2-equivalent tag in a non-55 container (127.57)' # bug 20
    build '{"name":"T127","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"127":{"type":"Composite","length":999,"description":"Private TLV","prefix":"ASCII.LLL","tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"57":{"type":"Binary","length":18,"description":"Track2Eq","enc":"Binary","prefix":"BerTLV"}}}}}' \
      '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"127.57":"4111111111111111D29122011234567890"}}'
    When run iso8583tool view "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '4111111111111111'
  End

  It 'masks a sensitive tag nested in a constructed TLV (55.70.57)' # bug 44
    build '{"name":"C57","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"55":{"type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL","tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"70":{"type":"Composite","length":255,"description":"Template","prefix":"BerTLV","tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"57":{"type":"Binary","length":18,"description":"Track2Eq","enc":"Binary","prefix":"BerTLV"}}}}}}}' \
      '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"55.70.57":"4111111111111111D29122011234567890"}}'
    When run iso8583tool redact "$WORK/m.hex" --spec "$WORK/spec.json" --format json
    The status should be success
    The output should not include '4111111111111111'
  End
End

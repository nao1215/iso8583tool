#!/bin/sh
# shellcheck shell=sh
#
# constructed (nested) TLV: a TLV tag whose value is itself a TLV template must
# round-trip to its leaf dot-path (55.70.9F02), be selectable with view --filter,
# and diff at the leaf tag, instead of collapsing into the parent tag's raw blob.

Describe 'iso8583tool nested TLV'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    make_workdir
    cat > "$WORK/spec.json" <<'JSON'
{
  "name": "Constructed TLV",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "11": {"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},
    "55": {
      "type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL",
      "tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},
      "subfields": {
        "70": {
          "type":"Composite","length":255,"description":"Template","prefix":"BerTLV",
          "tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},
          "subfields": {"9F02": {"type":"Binary","length":6,"description":"Amount","enc":"Binary","prefix":"BerTLV"}}
        }
      }
    }
  }
}
JSON
    printf '%s' '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"55.70.9F02":"000000005000"}}' > "$WORK/a.json"
    printf '%s' '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"55.70.9F02":"000000009999"}}' > "$WORK/b.json"
    iso8583tool convert "$WORK/a.json" --to hex --spec "$WORK/spec.json" --output "$WORK/a.hex" >/dev/null
    iso8583tool convert "$WORK/b.json" --to hex --spec "$WORK/spec.json" --output "$WORK/b.hex" >/dev/null
  }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'unpacks a nested TLV to its leaf path'
    When run iso8583tool convert "$WORK/a.hex" --spec "$WORK/spec.json"
    The status should be success
    The output should include '"55.70.9F02"'
    The output should not include '"55.70":'
  End

  It 'selects a nested TLV leaf with --filter'
    When run iso8583tool view "$WORK/a.hex" --spec "$WORK/spec.json" --filter 55.70.9F02 --no-color
    The status should be success
    The output should include '55.70.9F02'
    The output should not include '<not present>'
  End

  It 'diffs at the nested TLV leaf tag'
    When run iso8583tool diff "$WORK/a.hex" "$WORK/b.hex" --spec "$WORK/spec.json" --no-color
    The status should be success
    The output should include 'Field 55.70.9F02 changed'
  End

  It 'keeps a top-level tag and a nested tag set on the same field'
    # A message that mixes "55.82" (top-level) with "55.70.9F02" (constructed)
    # must round-trip both; the flat tag previously overwrote the whole composite.
    When run sh -c '
      printf "%s" "{\"mti\":\"0110\",\"fields\":{\"11\":\"123456\"},\"binary_fields\":{\"55.82\":\"3900\",\"55.70.9F02\":\"000000005000\"}}" > "$1/mix.json" &&
      "$ISO_BIN" convert "$1/mix.json" --to hex --spec "$1/spec.json" --output "$1/mix.hex" >/dev/null &&
      "$ISO_BIN" convert "$1/mix.hex" --spec "$1/spec.json"' sh "$WORK"
    The status should be success
    The output should include '"55.82"'
    The output should include '"55.70.9F02"'
  End
End

#!/bin/sh
# shellcheck shell=sh
#
# The full describe view must keep the parent path of any nested composite, and
# annotate a known leaf tag at any depth: a nested positional composite shows
# F48.2 / 48.2.1, and a constructed EMV tag (ARC 8A, date 9A) inside a template
# resolves its meaning even when nested several levels deep.

Describe 'iso8583tool view nested composites'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  Describe 'nested positional composite'
    pos_setup() {
      cat > "$WORK/spec.json" <<'JSON'
{
  "name": "F48 nested positional",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "11": {"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},
    "48": {
      "type":"Composite","length":999,"description":"Private Data","prefix":"ASCII.LLL",
      "tag":{"sort":"StringsByInt"},
      "subfields": {
        "1": {"type":"String","length":2,"description":"A","enc":"ASCII","prefix":"ASCII.Fixed"},
        "2": {
          "type":"Composite","length":6,"description":"B","prefix":"ASCII.Fixed",
          "tag":{"sort":"StringsByInt"},
          "subfields": {"1": {"type":"String","length":6,"description":"Nested","enc":"ASCII","prefix":"ASCII.Fixed"}}
        }
      }
    }
  }
}
JSON
      printf '%s' '{"mti":"0100","fields":{"11":"123456","48.1":"AB","48.2.1":"260604"}}' > "$WORK/msg.json"
      iso8583tool convert "$WORK/msg.json" --to hex --spec "$WORK/spec.json" --output "$WORK/msg.hex" >/dev/null
    }
    BeforeEach 'pos_setup'

    It 'keeps the nested positional path'
      When run iso8583tool view "$WORK/msg.hex" --spec "$WORK/spec.json" --encoding hex --no-color
      The status should be success
      The output should include 'F48.2'
      The output should include '48.2.1'
    End
  End

  Describe 'nested EMV tag annotation'
    emv_setup() {
      cat > "$WORK/spec.json" <<'JSON'
{
  "name": "Constructed TLV 8A",
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
          "subfields": {
            "8A": {"type":"Binary","length":2,"description":"ARC","enc":"Binary","prefix":"BerTLV"},
            "9A": {"type":"Binary","length":3,"description":"Txn Date","enc":"Binary","prefix":"BerTLV"}
          }
        }
      }
    }
  }
}
JSON
      printf '%s' '{"mti":"0110","fields":{"11":"123456"},"binary_fields":{"55.70.8A":"3030","55.70.9A":"260605"}}' > "$WORK/msg.json"
      iso8583tool convert "$WORK/msg.json" --to hex --spec "$WORK/spec.json" --output "$WORK/msg.hex" >/dev/null
    }
    BeforeEach 'emv_setup'

    It 'annotates a nested ARC tag as Approved'
      When run iso8583tool view "$WORK/msg.hex" --spec "$WORK/spec.json" --encoding hex --no-color
      The status should be success
      The output should include '55.70.8A'
      The output should include 'Approved'
    End

    It 'decodes nested leaf tags in validate --format json'
      When run iso8583tool validate "$WORK/msg.hex" --spec "$WORK/spec.json" --encoding hex --format json
      The status should be success
      The output should include '55.70.9A'
      The output should include '2026-06-05'
    End
  End
End

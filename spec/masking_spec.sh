#!/bin/sh
# shellcheck shell=sh
#
# Sensitive-data masking: view/redact/diff must mask a PAN/track wherever it
# appears — in additional-data fields, in a binary representation, behind a
# separator, in any TLV container at any depth — without over-masking a plain
# business identifier or a non-PAN field such as the country code (field 20).

Describe 'iso8583tool sensitive-data masking'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  view_json() { # $1 = json doc, $2.. = extra view flags
    doc="$1"; shift
    printf '%s' "$doc" | iso8583tool convert --to hex | iso8583tool view - --format json "$@"
  }

  It 'does not mask a non-PAN business identifier'
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","63":"ORDER_ID=1234567890123|TOKEN=ABC"}}'
    The status should be success
    The output should include 'ORDER_ID=1234567890123'
  End

  It 'masks a dash-separated PAN'
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","63":"PAN=4111-1111-1111-1111"}}'
    The status should be success
    The output should not include '1111-1111-1111'
  End

  It 'masks a space-separated PAN'
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","63":"PAN=4111 1111 1111 1111"}}'
    The status should be success
    The output should not include '1111 1111 1111'
  End

  It 'masks a PAN embedded in a non-private free-form field'
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","44":"PAN=4111111111111111"}}'
    The status should be success
    The output should not include '4111111111111111'
  End

  It 'masks the extended PAN field 34'
    When call view_json '{"mti":"0100","fields":{"11":"123456","34":"411111111111111111111111"}}'
    The status should be success
    The output should not include '411111111111111111111111'
  End

  It 'does not mask the country code field 20'
    When call view_json '{"mti":"0100","fields":{"11":"123456","20":"840"}}'
    The status should be success
    The output should include '"20": "840"'
  End

  It 'shows the raw field 20 change in diff'
    printf '%s' '{"mti":"0100","fields":{"11":"123456","20":"840"}}' | iso8583tool convert --to hex > "$WORK/a.hex"
    printf '%s' '{"mti":"0100","fields":{"11":"123456","20":"392"}}' | iso8583tool convert --to hex > "$WORK/b.hex"
    When run iso8583tool diff "$WORK/a.hex" "$WORK/b.hex" --no-color
    The status should be success
    The output should include '840'
    The output should include '392'
  End

  It 'masks a whole free-form track, not just its PAN'
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","63":"TRACK2=4111111111111111D29122011234567890"}}'
    The status should be success
    The output should not include '29122011234567890'
  End

  It 'masks an underscore-labeled PAN (card_no)'
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","63":"card_no=4222222222222222"}}'
    The status should be success
    The output should not include '4222222222222222'
  End

  It 'masks a spaced-label PAN (card number)'
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","63":"card number=4222222222222222"}}'
    The status should be success
    The output should not include '4222222222222222'
  End

  Describe 'a custom positional composite with a PAN-numbered subfield'
    sub_setup() {
      cat > "$WORK/spec.json" <<'JSON'
{
  "name": "F48 positional",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "11": {"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},
    "48": {
      "type":"Composite","length":999,"description":"Private Data","prefix":"ASCII.LLL",
      "tag":{"sort":"StringsByInt"},
      "subfields": {
        "1": {"type":"String","length":3,"description":"A","enc":"ASCII","prefix":"ASCII.Fixed"},
        "2": {"type":"String","length":2,"description":"B","enc":"ASCII","prefix":"ASCII.Fixed"}
      }
    }
  }
}
JSON
      printf '%s' '{"mti":"0100","fields":{"11":"123456","48.1":"ABC","48.2":"DE"}}' > "$WORK/msg.json"
      iso8583tool convert "$WORK/msg.json" --to hex --spec "$WORK/spec.json" --output "$WORK/msg.hex" >/dev/null
    }
    BeforeEach 'sub_setup'

    It 'does not mask subfield 48.2 with the top-level PAN rule'
      When run iso8583tool view "$WORK/msg.hex" --spec "$WORK/spec.json" --encoding hex --no-color
      The status should be success
      The output should include '48.2'
      The output should include 'DE'
    End
  End
End

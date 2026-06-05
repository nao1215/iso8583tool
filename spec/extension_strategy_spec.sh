#!/bin/sh
# shellcheck shell=sh
#
# The Extension Field Strategy section must describe how the active spec actually
# models a field. A bare custom --spec PATH is not BASE I, so it gets no built-in
# catalog at all; a built-in field modeled as a plain string is reported opaque,
# not by the catalog's positional/bitmap assumption; and setting a dot-path on a
# plain built-in field fails with a clear explanation.

Describe 'iso8583tool extension strategy'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  Describe 'a custom positional composite spec'
    pos_setup() {
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
    BeforeEach 'pos_setup'

    It 'does not apply the BASE I catalog to a custom spec'
      When run iso8583tool view "$WORK/msg.hex" --spec "$WORK/spec.json" --encoding hex --no-color
      The status should be success
      The output should not include 'Extension Field Strategy:'
      The output should not include 'Additional Data - Private'
      The output should not include '[tlv]'
    End
  End

  Describe 'a built-in plain field documented as bitmap'
    f127_setup() {
      printf '%s' '{"mti":"0100","fields":{"11":"123456","127":"EEE"}}' > "$WORK/msg.json"
      iso8583tool convert "$WORK/msg.json" --to hex --output "$WORK/msg.hex" >/dev/null
    }
    BeforeEach 'f127_setup'

    It 'reports field 127 as opaque, matching the spec'
      When run iso8583tool view "$WORK/msg.hex" --no-color
      The status should be success
      The output should include 'F127 Reserved Private [opaque]'
      The output should not include 'F127 Reserved Private [bitmap]'
    End
  End

  It 'explains a dot-path set on a plain built-in field'
    When run sh -c 'printf "%s" "{\"mti\":\"0100\",\"fields\":{\"11\":\"123456\",\"48.1\":\"AB\"}}" | "$ISO_BIN" convert --to hex'
    The status should be failure
    The stderr should include 'dot-path subfields'
    The stderr should not include 'PathMarshaler'
  End
End

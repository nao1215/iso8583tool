#!/bin/sh
# shellcheck shell=sh
#
# A field value carrying raw ANSI/control bytes must not be emitted verbatim by
# any text view: view, validate, diff, and redact escape it (caret notation,
# like `cat -v`) so it cannot drive the terminal. The poisoned message is built
# under a custom spec where field 41 is Binary, then read under basei-starter,
# where field 41 is plain text.

Describe 'iso8583tool control-byte sanitization'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    make_workdir
    cat > "$WORK/bin41.json" <<'JSON'
{"name":"Bin41","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"41":{"type":"Binary","length":8,"description":"Terminal","enc":"Binary","prefix":"Binary.Fixed"}}}
JSON
    printf '%s' '{"mti":"0100","binary_fields":{"41":"1B5B324A20202020"}}' | iso8583tool convert --to hex --spec "$WORK/bin41.json" > "$WORK/a.hex"
    printf '%s' '{"mti":"0100","binary_fields":{"41":"1B5B316D20202020"}}' | iso8583tool convert --to hex --spec "$WORK/bin41.json" > "$WORK/b.hex"
  }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  # An ESC byte (0x1b) is printed by printf as the literal escape; the assertion
  # checks it is NOT present in the output (it is escaped to "^[" instead).
  esc() { printf '\033'; }

  It 'view escapes control bytes'
    When run iso8583tool view "$WORK/a.hex" --no-color
    The status should be success
    The output should not include "$(esc)"
    The output should include '^['
  End

  It 'validate escapes control bytes'
    When run iso8583tool validate "$WORK/a.hex" --no-color
    The status should be success
    The output should not include "$(esc)"
  End

  It 'diff escapes control bytes'
    When run iso8583tool diff "$WORK/a.hex" "$WORK/b.hex" --no-color
    The status should be success
    The output should not include "$(esc)"
    The output should include '^['
  End

  It 'redact text escapes control bytes'
    When run iso8583tool redact "$WORK/a.hex" --format text --no-color
    The status should be success
    The output should not include "$(esc)"
    The output should include '^['
  End
End

#!/bin/sh
# shellcheck shell=sh
#
# doctor: detect which built-in preset fits a message, across ASCII and packed
# BCD, and the no-fit / validate-hint paths.

Describe 'iso8583tool doctor'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'recommends the BASE I starter for an ASCII BASE I message'
    When run iso8583tool doctor "$EXAMPLES/0110-auth-response.hex"
    The status should be success
    The output should include 'Recommended: --spec basei-starter'
    The output should include 'Confirm with: iso8583tool view'
  End

  It 'detects a packed-BCD raw message'
    make_workdir
    write_kanmu_like_message "$WORK/message.bin"
    When run iso8583tool doctor "$WORK/message.bin" --encoding raw
    The status should be success
    The output should include "Recommended: --spec $PACKED_BCD_SPEC"
    remove_workdir
  End

  It 'auto-detects a raw .bin without --encoding'
    make_workdir
    write_kanmu_like_message "$WORK/message.bin"
    When run iso8583tool doctor "$WORK/message.bin"
    The status should be success
    The output should include '(raw input)'
    The output should include "Recommended: --spec $PACKED_BCD_SPEC"
    remove_workdir
  End

  It 'emits a JSON report with --format json'
    When run iso8583tool doctor "$EXAMPLES/0110-auth-response.hex" --format json
    The status should be success
    The output should include '"recommended": "basei-starter"'
    The output should include '"exact_round_trip": true'
  End

  It 'exits non-zero when no preset fits'
    When run iso8583tool doctor --raw fffefd
    The status should be failure
    The output should include 'No built-in preset could unpack'
  End

  It 'is suggested by a wrong-spec validate failure'
    # A complete message that fails at a data field under the wrong spec is the
    # case the doctor hint is for; a header-level failure is reported as
    # truncated/corrupt instead.
    When run iso8583tool validate "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --spec spec87bcd-starter
    The status should be failure
    The output should include 'doctor'
  End

  It 'marks every tied preset recommended and confirms with each'
    When run iso8583tool doctor "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --no-color
    The status should be success
    The output should include 'view --spec basei-starter'
    The output should include 'view --spec spec87ascii'
  End

  Describe 'shell-safe confirm hint'
    BeforeEach 'make_workdir; cp "$EXAMPLES/0110-auth-response.hex" "$WORK/with space.hex"'
    AfterEach 'remove_workdir'

    It 'quotes a path that contains a space'
      When run iso8583tool doctor "$WORK/with space.hex" --no-color
      The status should be success
      The output should include "'"
      The output should include 'with space.hex'
    End
  End

  Describe 'custom-spec validate hint'
    custom_setup() {
      make_workdir
      printf '%s' '{"name":"F48 positional","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"48":{"type":"Composite","length":999,"description":"Private Data","prefix":"ASCII.LLL","tag":{"sort":"StringsByInt"},"subfields":{"1":{"type":"String","length":3,"description":"A","enc":"ASCII","prefix":"ASCII.Fixed"},"2":{"type":"String","length":2,"description":"B","enc":"ASCII","prefix":"ASCII.Fixed"}}}}}' > "$WORK/spec.json"
      printf '%s' '{"name":"F127 bitmap","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"127":{"type":"Composite","length":255,"description":"Private use field","prefix":"ASCII.LL","bitmap":{"type":"Bitmap","length":8,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed","disableAutoExpand":true},"subfields":{"1":{"type":"String","length":2,"description":"A","enc":"ASCII","prefix":"ASCII.Fixed"},"2":{"type":"String","length":2,"description":"B","enc":"ASCII","prefix":"ASCII.Fixed"}}}}}' > "$WORK/other.json"
      printf '%s' '{"mti":"0100","fields":{"11":"123456","127.1":"AA","127.2":"BB"}}' | iso8583tool convert --to hex --spec "$WORK/other.json" > "$WORK/msg.hex"
    }
    BeforeEach 'custom_setup'
    AfterEach 'remove_workdir'

    It 'does not steer a custom-spec failure to doctor'
      When run iso8583tool validate "$WORK/msg.hex" --spec "$WORK/spec.json" --encoding hex --no-color
      The status should be failure
      The output should not include 'doctor'
      The output should include 'spec file'
    End
  End
End

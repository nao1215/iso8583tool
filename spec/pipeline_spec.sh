#!/bin/sh
# shellcheck shell=sh
#
# Real user workflows, end to end: chaining commands through pipes, editing an
# EMV tag and packing it back, preserving an unknown tag across a round trip,
# and extracting a single field value for a script.

Describe 'iso8583tool workflows'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'streams sample -> convert -> view'
    When run sh -c '"$ISO_BIN" sample 0100-auth-request --format hex | "$ISO_BIN" convert | "$ISO_BIN" convert | "$ISO_BIN" view -'
    The status should be success
    The output should include 'MTI'
  End

  Describe 'editing an EMV tag' edit
    BeforeEach 'make_workdir'
    AfterEach 'remove_workdir'

    It 'edits one tag and packs it back'
      iso8583tool convert "$EXAMPLES/0100-auth-request.hex" > "$WORK/msg.json"
      sed 's/"55.9F02": "000000005000"/"55.9F02": "000000010000"/' "$WORK/msg.json" > "$WORK/edited.json"
      When run sh -c '"$ISO_BIN" convert "$WORK/edited.json" | "$ISO_BIN" view - --filter 55.9F02'
      The status should be success
      The output should include '000000010000'
    End

    It 'keeps an unknown tag through the round trip'
      iso8583tool convert "$EXAMPLES/0100-auth-request-unknown-tlv.hex" > "$WORK/u.json"
      When run sh -c '"$ISO_BIN" convert "$WORK/u.json" | "$ISO_BIN" validate -'
      The status should be success
      The output should include '55.DF8129'
    End
  End

  It 'extracts a single field value with --filter'
    When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --filter 39
    The status should be success
    The output should include '00'
    The output should include 'Approved'
  End
End

#!/bin/sh
# shellcheck shell=sh
#
# validate --strict must not pass a hollow advice or network-management message:
# an advice carries the same core data elements as the request it stands in for,
# and every network-management message identifies its purpose with field 70.

Describe 'iso8583tool validate --strict advice and network rules'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  # hollow packs a near-empty message for the given MTI and prints its hex path.
  hollow() {
    printf '%s' "$2" > "$WORK/$1.json"
    iso8583tool convert "$WORK/$1.json" --to hex --output "$WORK/$1.hex" >/dev/null
    printf '%s' "$WORK/$1.hex"
  }

  It 'fails a hollow authorization advice (0120)'
    hex=$(hollow 0120 '{"mti":"0120","fields":{"11":"123456"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'failed'
  End

  It 'fails a hollow financial advice (0220)'
    hex=$(hollow 0220 '{"mti":"0220","fields":{"11":"123456"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'failed'
  End

  It 'fails a hollow network advice (0820)'
    hex=$(hollow 0820 '{"mti":"0820","fields":{"11":"123456"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'failed'
  End

  It 'fails a hollow network response (0810)'
    hex=$(hollow 0810 '{"mti":"0810","fields":{"11":"123456","39":"00"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'failed'
  End

  It 'fails a hollow network advice response (0830)'
    hex=$(hollow 0830 '{"mti":"0830","fields":{"11":"123456","39":"00"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'failed'
  End

  It 'still accepts the bundled network echo under --strict'
    When run iso8583tool validate "$EXAMPLES/0800-network-echo.hex" --strict
    The status should be success
    The output should include 'ok'
  End
End

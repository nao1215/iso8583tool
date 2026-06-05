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

  It 'fails a hollow authorization notification (0140)'
    hex=$(hollow 0140 '{"mti":"0140","fields":{"11":"123456"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'failed'
  End

  It 'fails a hollow financial instruction ack (0270)'
    hex=$(hollow 0270 '{"mti":"0270","fields":{"11":"123456"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'failed'
  End

  It 'fails a hollow file-action request (0300)'
    hex=$(hollow 0300 '{"mti":"0300","fields":{"11":"123456"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'failed'
  End

  It 'requires a PAN source for a reversal request (0400)'
    hex=$(hollow 0400 '{"mti":"0400","fields":{"4":"000000001000","7":"0605123456","11":"123456","90":"020022334406041301050000000000000000000000"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'PAN source'
  End

  It 'warns that reconciliation (0500) rules are not implemented'
    hex=$(hollow 0500 '{"mti":"0500","fields":{"11":"123456"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be success
    The output should include 'class 5'
  End

  It 'rejects an alphabetic value in a numeric field (70)'
    hex=$(hollow c0800 '{"mti":"0800","fields":{"11":"123456","70":"ABC"}}')
    When run iso8583tool validate "$hex" --strict
    The status should be failure
    The output should include 'must be numeric'
  End
End

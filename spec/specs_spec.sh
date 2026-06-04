#!/bin/sh
# shellcheck shell=sh
#
# specs: list the built-in spec presets in text and JSON.

Describe 'iso8583tool specs'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'lists the built-in presets with the default marked'
    When run iso8583tool specs
    The status should be success
    The output should include 'basei-starter (default)'
    The output should include 'spec87ascii'
    The output should include 'spec87bcd-starter'
  End

  It 'emits a JSON array with --format json'
    When run iso8583tool specs --format json
    The status should be success
    The output should include '"name": "basei-starter"'
    The output should include '"default": true'
  End

  It 'rejects an unexpected positional argument'
    When run iso8583tool specs extra
    The status should be failure
    The error should include 'Usage:'
  End
End

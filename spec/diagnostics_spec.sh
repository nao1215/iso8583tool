#!/bin/sh
# shellcheck shell=sh
#
# Detection messaging: doctor must not present the default preset as the single
# answer when more than one fits equally well, and both doctor and validate must
# call out a truncated/malformed capture instead of steering the user to a custom
# spec or to doctor when neither will help.

Describe 'iso8583tool detection messaging'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'presents tied presets for an ambiguous message' # bug 22
    When run iso8583tool doctor "$PROJECT_ROOT/examples/spec87ascii/0800-network-echo.hex" --no-color
    The status should be success
    The output should include 'spec87ascii'
    The output should include 'fits equally well'
  End

  It 'flags a truncated capture instead of only "custom layout"' # bug 39
    When run iso8583tool doctor --raw 010000000000000008000103DF --no-color
    The status should be failure
    The output should include 'truncated or malformed'
  End

  It 'validate calls out a truncated capture rather than doctor' # bug 40
    When run iso8583tool validate --raw 010000000000000008000103DF --no-color
    The status should be failure
    The output should include 'truncated or malformed'
    The output should not include 'iso8583tool doctor'
  End
End

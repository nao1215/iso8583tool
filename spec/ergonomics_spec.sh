#!/bin/sh
# shellcheck shell=sh
#
# Everyday ergonomics: flags before or after the positional target, repeated
# --filter, color modes (auto/always/never/--no-color and NO_COLOR), and config
# selection.

Describe 'iso8583tool ergonomics'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'flag ordering'
    It 'accepts the target after the flags'
      When run iso8583tool view --format json "$EXAMPLES/0110-auth-response.hex"
      The status should be success
      The output should include '"mti"'
    End

    It 'accepts the target before the flags'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --format json
      The status should be success
      The output should include '"mti"'
    End

    It 'accepts flags interleaved around the target'
      When run iso8583tool view --filter 39 "$EXAMPLES/0110-auth-response.hex" --filter 49
      The status should be success
      The output should include 'Approved'
      The output should include 'JPY'
    End
  End

  Describe 'color'
    # When captured (not a tty) auto stays off, so a plain run has no escapes.
    It 'is plain by default when not on a terminal'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex"
      The status should be success
      The output should not include "$(printf '\033')"
    End

    It 'forces color with --color always'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --color always
      The status should be success
      The output should include "$(printf '\033')"
    End

    It 'stays plain with --no-color even when forced elsewhere'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --no-color
      The status should be success
      The output should not include "$(printf '\033')"
    End

    It 'rejects an unknown --color value instead of ignoring it'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --color banana
      The status should be failure
      The stderr should include 'invalid --color'
    End
  End

  Describe 'end of options'
    BeforeEach 'make_workdir'
    AfterEach 'remove_workdir'

    It 'treats a dash-leading filename after -- as a positional'
      cp "$EXAMPLES/0110-auth-response.hex" "$WORK/-response.hex"
      When run sh -c 'cd "$WORK" && "$ISO_BIN" view -- -response.hex'
      The status should be success
      The output should include 'MTI'
    End
  End

  Describe 'config'
    BeforeEach 'make_workdir'
    AfterEach 'remove_workdir'

    It 'applies an extension catalog from --config'
      printf '%s' '{"spec":"basei-starter","extensions":[{"id":63,"name":"Acme Blob","strategy":"opaque"}]}' > "$WORK/cfg.json"
      When run iso8583tool validate "$EXAMPLES/0110-auth-response.hex" --config "$WORK/cfg.json"
      The status should be success
      The output should include 'Acme Blob'
    End

    It 'fails on a config with an invalid strategy'
      printf '%s' '{"extensions":[{"id":1,"strategy":"nope"}]}' > "$WORK/bad.json"
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --config "$WORK/bad.json"
      The status should be failure
      The stderr should include 'strategy'
    End
  End
End

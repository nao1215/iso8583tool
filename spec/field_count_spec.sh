#!/bin/sh
# shellcheck shell=sh
#
# convert --output summary: the "packed N fields" count must report top-level
# ISO fields, the same number doctor reports as field_count, instead of counting
# every TLV subtag as a separate field.

Describe 'iso8583tool convert field count'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'reports the top-level field count matching doctor'
    out="$WORK/out.hex"
    summary=$(iso8583tool convert "$EXAMPLES/0100-auth-request.json" --output "$out" | head -1)
    count=$(iso8583tool doctor "$out" --format json \
      | sed -n 's/.*"field_count": \([0-9]*\).*/\1/p' | head -1)
    When call test "$summary" = "Converted with basei-starter (packed $count fields to hex)."
    The status should be success
  End
End

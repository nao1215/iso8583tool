#!/bin/sh
# shellcheck shell=sh
#
# Sensitive-data masking: view/redact/diff must mask a PAN/track wherever it
# appears — in additional-data fields, in a binary representation, behind a
# separator, in any TLV container at any depth — without over-masking a plain
# business identifier or a non-PAN field such as the country code (field 20).

Describe 'iso8583tool sensitive-data masking'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  view_json() { # $1 = json doc, $2.. = extra view flags
    doc="$1"; shift
    printf '%s' "$doc" | iso8583tool convert --to hex | iso8583tool view - --format json "$@"
  }

  It 'does not mask a non-PAN business identifier' # bug 24
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","63":"ORDER_ID=1234567890123|TOKEN=ABC"}}'
    The status should be success
    The output should include 'ORDER_ID=1234567890123'
  End

  It 'masks a dash-separated PAN' # bug 25
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","63":"PAN=4111-1111-1111-1111"}}'
    The status should be success
    The output should not include '1111-1111-1111'
  End

  It 'masks a space-separated PAN' # bug 26
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","63":"PAN=4111 1111 1111 1111"}}'
    The status should be success
    The output should not include '1111 1111 1111'
  End

  It 'masks a PAN embedded in a non-private free-form field' # bug 27
    When call view_json '{"mti":"0110","fields":{"11":"123456","39":"00","44":"PAN=4111111111111111"}}'
    The status should be success
    The output should not include '4111111111111111'
  End

  It 'masks the extended PAN field 34' # bug 28
    When call view_json '{"mti":"0100","fields":{"11":"123456","34":"411111111111111111111111"}}'
    The status should be success
    The output should not include '411111111111111111111111'
  End

  It 'does not mask the country code field 20' # bug 29
    When call view_json '{"mti":"0100","fields":{"11":"123456","20":"840"}}'
    The status should be success
    The output should include '"20": "840"'
  End

  It 'shows the raw field 20 change in diff' # bug 30
    printf '%s' '{"mti":"0100","fields":{"11":"123456","20":"840"}}' | iso8583tool convert --to hex > "$WORK/a.hex"
    printf '%s' '{"mti":"0100","fields":{"11":"123456","20":"392"}}' | iso8583tool convert --to hex > "$WORK/b.hex"
    When run iso8583tool diff "$WORK/a.hex" "$WORK/b.hex" --no-color
    The status should be success
    The output should include '840'
    The output should include '392'
  End
End

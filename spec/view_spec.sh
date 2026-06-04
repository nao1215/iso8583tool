#!/bin/sh
# shellcheck shell=sh
#
# view: decoding, the one-line summary, PAN masking, --filter, JSON output, and
# reading from stdin.

Describe 'iso8583tool view'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'describe output'
    It 'decodes codes and prints a summary'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex"
      The status should be success
      The output should include 'Summary:'
      The output should include 'Approved'
      The output should include 'JPY 5000'
      The output should include '06-04 12:34:56'
    End

    It 'masks the PAN'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex"
      The status should be success
      The output should include '411111******1111'
      The output should not include '4111111111111111'
    End
  End

  Describe 'json output'
    It 'emits a decoded array and stays uncolored'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --format json --color always
      The status should be success
      The output should include '"decoded"'
      The output should include '"meaning": "Approved"'
    End
  End

  Describe '--filter'
    It 'prints only the requested fields'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --filter 39 --filter 55.8A
      The status should be success
      The output should include 'Approved'
      The output should not include 'Primary Account Number'
    End

    It 'marks a field that is not present'
      When run iso8583tool view "$EXAMPLES/0110-auth-response.hex" --filter 90
      The status should be success
      The output should include 'not present'
    End
  End

  Describe 'stdin'
    It 'reads a message piped in via -'
      When run sh -c '"$ISO_BIN" sample 0110-auth-response --format hex | "$ISO_BIN" view -'
      The status should be success
      The output should include 'MTI'
      The output should include 'Approved'
    End

    It 'reads from stdin when the target is omitted'
      When run sh -c '"$ISO_BIN" sample 0110-auth-response --format hex | "$ISO_BIN" view'
      The status should be success
      The output should include 'MTI'
    End
  End

  Describe 'private-field safety'
    It 'masks a PAN embedded in a free-form private field by default'
      When run sh -c 'printf "%s" "{\"mti\":\"0110\",\"fields\":{\"11\":\"123456\",\"39\":\"00\",\"63\":\"PAN=4111111111111111\"}}" | "$ISO_BIN" convert --to hex | "$ISO_BIN" view - --format json'
      The status should be success
      The output should not include '4111111111111111'
    End

    It 'reveals the raw private-field value with --unsafe'
      When run sh -c 'printf "%s" "{\"mti\":\"0110\",\"fields\":{\"11\":\"123456\",\"39\":\"00\",\"63\":\"PAN=4111111111111111\"}}" | "$ISO_BIN" convert --to hex | "$ISO_BIN" view - --format json --unsafe'
      The status should be success
      The output should include '4111111111111111'
    End
  End
End

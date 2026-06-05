#!/bin/sh
# shellcheck shell=sh
#
# custom JSON spec import: moov-io/iso8583 field types that the upstream JSON
# importer does not wire up by default (Hex, Track1, Track3, IndexTag) and tag
# blocks that omit "sort" must load with --spec PATH instead of failing with a
# "no constructor" / "unknown sort function" error.
#
# Most specs below define only the field under test, so the bundled example
# message does not unpack under them and the command exits non-zero. That is
# expected: the point of each assertion is that the failure is NOT the import
# error, which proves the spec itself loaded.

Describe 'iso8583tool custom JSON spec import'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() { make_workdir; }
  cleanup() { remove_workdir; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  # write_doc writes a one-field JSON document so a custom spec can be exercised
  # end to end via convert.
  write_doc() {
    printf '%s' '{"mti":"0100","fields":{"0":"0100"},"binary_fields":{"52":"A1B2C3D4E5F60708"}}' > "$WORK/doc.json"
  }

  It 'loads a top-level Hex field and round-trips it'
    spec="$WORK/hex-top.json"
    printf '%s' '{"name":"Hex top","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"52":{"type":"Hex","length":8,"description":"PIN Data","enc":"Binary","prefix":"Binary.Fixed"}}}' > "$spec"
    write_doc
    When run iso8583tool convert "$WORK/doc.json" --to hex --spec "$spec"
    The status should be success
    The output should not include 'no constructor'
  End

  It 'loads a Hex TLV subfield'
    spec="$WORK/hex-sub.json"
    printf '%s' '{"name":"TLV Hex","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"55":{"type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL","tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"9F02":{"type":"Hex","length":6,"description":"Amount","enc":"Binary","prefix":"BerTLV"}}}}}' > "$spec"
    When run iso8583tool view "$EXAMPLES/0100-auth-request.hex" --spec "$spec"
    The status should be failure
    The error should not include 'no constructor'
  End

  It 'loads a Track1 field'
    spec="$WORK/track1.json"
    printf '%s' '{"name":"Track1","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"45":{"type":"Track1","length":76,"description":"Track 1","enc":"ASCII","prefix":"ASCII.LL"}}}' > "$spec"
    When run iso8583tool view "$EXAMPLES/0100-auth-request.hex" --spec "$spec"
    The status should be failure
    The error should not include 'no constructor'
  End

  It 'loads a Track3 field'
    spec="$WORK/track3.json"
    printf '%s' '{"name":"Track3","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"36":{"type":"Track3","length":104,"description":"Track 3","enc":"ASCII","prefix":"ASCII.LLL"}}}' > "$spec"
    When run iso8583tool view "$EXAMPLES/0100-auth-request.hex" --spec "$spec"
    The status should be failure
    The error should not include 'no constructor'
  End

  It 'loads an IndexTag composite subfield'
    spec="$WORK/indextag.json"
    printf '%s' '{"name":"IndexTag","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"48":{"type":"Composite","length":999,"description":"IndexTag Composite","prefix":"ASCII.LLL","tag":{"sort":"StringsByInt","length":2,"enc":"ASCII"},"subfields":{"1":{"type":"IndexTag","length":2,"description":"Tag index","enc":"ASCII","prefix":"ASCII.Fixed"}}}}}' > "$spec"
    When run iso8583tool view "$EXAMPLES/0100-auth-request.hex" --spec "$spec"
    The status should be failure
    The error should not include 'no constructor'
  End

  It 'loads a composite tag that omits sort'
    spec="$WORK/nosort.json"
    printf '%s' '{"name":"No sort","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"55":{"type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL","tag":{"enc":"BerTLVTag","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},"subfields":{"9F02":{"type":"Binary","length":6,"description":"Amount","enc":"Binary","prefix":"BerTLV"}}}}}' > "$spec"
    When run iso8583tool view "$EXAMPLES/0100-auth-request.hex" --spec "$spec"
    The status should be failure
    The error should not include 'unknown sort function'
  End
End

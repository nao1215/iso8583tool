# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `view`: unpack and inspect a message. Numeric codes (MTI, response code,
  currency, amount, dates, and EMV tags) are translated to text, a one-line
  summary is printed, and PAN and track data are masked. `--filter PATH`
  (repeatable) prints only selected fields, `--format json` emits a machine
  readable report, and color is automatic on a terminal.
- `convert`: convert between a packed BASE I message and a JSON document, with
  the direction auto-detected from the input (`--to json|hex` to force it).
  Field 55 is emitted per EMV tag so individual tags are editable, and unknown
  tags are preserved across a round trip.
- `validate`: report whether a message unpacks, plus unknown TLV tags and
  extension-field strategy. A failure names the field that could not be
  unpacked. Exit code is 0 for warnings only and 1 on an error.
- `sample`: list and export the bundled BASE I fixtures.
- A single optional `--config` JSON file selects the spec (`basei-starter`,
  `spec87ascii`, or a moov-io/iso8583 JSON spec) and overrides the
  extension-field catalog.
- Messages can be read from a file, from `-`, or from stdin, so the commands
  compose in pipes.
- End-to-end tests (shellspec) that drive the built binary, and CI workflows for
  build, multi-platform unit tests, coverage (octocov), linting (golangci-lint
  via reviewdog), and e2e.

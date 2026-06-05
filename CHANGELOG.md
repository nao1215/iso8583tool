# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The built-in presets now define the standard ISO 8583:1987 secondary-bitmap
  fields (66-128). A document that uses common high-numbered fields — for example
  95 (replacement amounts), 96 (message security code), 100 (receiving
  institution id), 102-104 (account ids / transaction description), or the
  reserved range 123-128 — now packs and round-trips instead of failing with
  "field N is not defined in the spec". The fields the default extension catalog
  documents (including 126 and 127) are now backed by a definition in every
  built-in preset.

### Fixed

- The bundled `examples/spec87ascii/0800-network-echo` sample now carries the
  network-management code in field 70, so it passes `validate --strict` under its
  intended `spec87ascii` preset instead of failing for a missing field 70.

- Help requested on the success path now prints to stdout instead of stderr:
  `--help`, `help <command>`, `<command> --help`, and the no-argument overview.
  Error-path usage (an unknown command, a bad flag, or trailing arguments) still
  goes to stderr with a non-zero exit, matching common CLI convention.

- Sensitive-data masking now covers cases it previously missed, across `view`,
  `redact`, and `diff`:
  - A PAN or track carried in a binary (hex-encoded) field — for example a
    binary field 2, field 35, or a private field 63 holding `PAN=...` bytes — is
    now masked, not only the text representation.
  - Sensitive EMV/TLV tags (`5A`, `56`, `57`, `99`, `9F1F`, `9F20`, and now
    `9F6B` Track 2 Equivalent Data) are masked in any TLV container (`55`, `127`,
    …) and at any nesting depth (`55.70.57`), whether or not the active spec
    defines the tag.
  - Free-form additional-data fields (for example 44 and 54) are content-scanned
    for an embedded PAN, and embedded PANs written with space or hyphen grouping
    (`4111 1111 1111 1111`, `4111-1111-...`) are masked.
  - Field 34 (Extended PAN) is masked; field 20 is no longer masked (in the 1987
    layout it is the PAN Extended Country Code, not a secondary PAN), so its real
    value shows in `view`, `redact`, and `diff`.
  - The embedded-PAN scanner no longer masks plain numeric identifiers: a
    candidate is masked only when its digits pass the Luhn check or it follows a
    PAN-ish key label, so `ORDER_ID=1234567890123` is left intact.
- `view` reports a field's extension strategy as `tlv` when the active spec
  models it as a BER-TLV composite, instead of the built-in catalog default
  (which described, for example, field 127 as a nested bitmap).

- A constructed (nested) BER-TLV tag — a TLV tag whose value is itself a TLV
  template — now expands to its leaf dot-path (for example `55.70.9F02`) when a
  message is unpacked. It previously collapsed into the parent tag's raw blob
  (`55.70`), so `convert` lost the child path, `view --filter 55.70.9F02`
  reported it as `<not present>`, and `diff` reported the change against the
  parent blob. `view --filter` and `diff` now resolve the leaf path too.

- `doctor` no longer presents the default preset as the single answer when more
  than one preset fits a message equally well. The ambiguous presets are listed
  together (`--spec basei-starter or --spec spec87ascii`) with the
  "confirm by eye" note.
- `doctor` and `validate` now call out a truncated or corrupt capture instead of
  steering the user to a custom spec or to `doctor` when neither can help. When a
  message is too short to hold even its MTI/bitmap, `validate` reports it as
  truncated/malformed, and `doctor` says so when every preset fails the same way.

### Changed

- `view`, `validate`, `convert`, `diff`, and `redact` now default to
  `--encoding auto`, the same fit-based hex/raw detection `doctor` uses. A raw
  `*.bin` capture, and an all-numeric raw ASCII message that is byte-for-byte a
  valid hex string, are read correctly without `--encoding raw`. An explicit
  `--encoding hex|raw` still overrides the detection.

### Fixed

- `convert --output` now reports the top-level ISO field count in its
  `packed N fields` summary, matching the `field_count` `doctor` reports for the
  same message. It previously counted every TLV subtag (for example each
  `55.<tag>`) as a separate field, so the summary disagreed with `doctor`.

- `validate --strict` now checks advice and network-management messages that
  previously passed with only a STAN. An authorization/financial advice (`0120`,
  `0220`) is held to the same core requirements as the request it stands in for
  (processing code, amount, transmission date/time, and a PAN source), and every
  network-management message (`0810`, `0820`, `0830`, in addition to `0800`)
  requires the network-management code in field 70.

- `view --filter` and `diff --filter` now match EMV tag paths case-insensitively,
  so `55.9f02` and `55.9F02` select the same field instead of one of them
  reporting `<not present>` or `No differences.`.
- `diff --filter 0` now selects the MTI, matching `view` and `diff --filter mti`.
- `diff` now reports a filter that matched no field in either message (in text as
  `No field matched filter: ...` and in JSON as `missing_filters`), so a typo is
  distinguishable from a real no-change result.

- `convert` now tolerates a UTF-8 BOM (`EF BB BF`) at the start of a JSON
  document. Auto-detection no longer mistakes a BOM-prefixed object for hex, and
  an explicit `--to hex` no longer fails JSON parsing on the BOM. Editors and
  some exporters prepend the BOM even though it is not valid JSON.

- Zero-padded fixed-length fields now show their canonical width consistently
  across every surface. The full `view` describe output and the `decoded[].value`
  entries (in both `view --format json` and `validate --format json`) previously
  used the collapsed integer form (`F3: 0`, `value: "0"`) while the filtered and
  JSON `fields` views showed the padded form (`000000`). All of them now report
  the canonical, edit-ready value.

- A custom moov-io/iso8583 JSON spec passed with `--spec PATH` now loads the
  `Hex`, `Track1`, `Track3`, and `IndexTag` field types, both at the top level
  and inside composite subfields. Previously these failed with a "no constructor
  for field type" error even though moov-io exports the field types, because the
  upstream JSON importer does not register them. `IndexTag` is read as a
  positional string subfield (moov-io has no field type backing that name).
- A composite `tag` block that omits `sort` now loads instead of failing with
  "unknown sort function"; an omitted sort defaults to hex-tag order, which
  suits the BER-TLV composites these specs describe.
- The `spec87bcd-starter` preset now matches its packed-BCD intent. Field 55 is
  an EMV BER-TLV composite, so `55.<tag>` packs and round-trips instead of
  failing with "field 55 is not a PathMarshaler". The PIN (52) and MAC (64)
  fields are raw fixed-length binary, so they no longer fail to encode their
  length. Variable-length fields (for example 32, 35, 36, 45) encode their
  length prefix as BCD rather than ASCII, so a packed-BCD capture no longer
  carries an ASCII length pair such as `0x30 0x36` for a six-long field.

## [0.2.2] - 2026-06-05

### Fixed

- `doctor --encoding auto` no longer mis-reads a raw ASCII capture as hex text.
  An all-numeric message is, byte-for-byte, a valid even-length hex string, so
  the old "looks like hex" check picked the hex reading and recommended the
  wrong preset. Auto-detection now compares how well a built-in preset fits each
  reading and keeps the stronger one, so a raw ASCII message is detected as raw.
- An explicit empty `extensions` array in a config now disables the built-in
  extension catalog, matching the documented "the list replaces it" contract.
  Previously `{"extensions": []}` still fell back to the built-in catalog;
  omitting the key (the documented way to keep the built-in catalog) is
  unchanged.
- `view --filter` JSON no longer breaks the document shape for the MTI. The MTI
  stays top-level and is selected by field `0` or `mti`; it is never duplicated
  into `fields`, and `--filter 0` no longer reports it as missing.

## [0.2.1] - 2026-06-05

### Fixed

- `doctor` now auto-detects the input encoding (hex text vs raw bytes), so a raw
  `*.bin` capture works without `--encoding raw`. Previously `doctor message.bin`
  failed with a hex-decode error, which defeated the point of a detection tool.
  The detected encoding is shown, and the `Confirm with:` hint carries
  `--encoding raw` when needed. Override with `--encoding hex|raw`.
- A hex-decode failure now suggests `--encoding raw` for a raw binary message
  instead of only printing the low-level decode error.

## [0.2.0] - 2026-06-05

### Added

- `doctor`: detect which built-in spec preset fits a message. It tries every
  preset and recommends the best fit, ranked by an exact byte-length round trip,
  a clean unpack, a valid MTI, and the number of decoded fields. Text and
  `--format json` output; exits non-zero when no built-in preset can unpack the
  message.
- `specs`: list the built-in spec presets (`basei-starter`, `spec87ascii`,
  `spec87bcd-starter`) with their encoding and a one-line "when to use" note, in
  text or `--format json`.
- `--spec NAME|PATH` flag on `view`, `diff`, `redact`, `convert`, and `validate`
  to select a built-in preset or a `moov-io/iso8583` JSON spec directly. When
  both are given, `--spec` overrides the spec named in `--config`.
- `validate` now prints a `Hint` pointing at `doctor` when a message fails to
  unpack, since the usual cause is the wrong spec.

### Changed

- `--config` is now scoped to extension catalogs and default bundles; the spec is
  selected with `--spec`. The `spec` field in a config file still provides a
  default. The single-preset example configs (`spec87ascii.config.json`,
  `spec87bcd.config.json`) are removed, and the remaining examples are renamed to
  `examples/basei-overlay-config.json` and `examples/iso8583tool-config.json`.

## [0.1.0] - 2026-06-04

First public release: a command-line tool for debugging and inspecting ISO 8583
payment messages, oriented around BASE I.

### Added

- `view`: unpack and inspect a message. Numeric codes (MTI, response code,
  currency, amount, dates, and EMV tags) are translated to text, a one-line
  summary is printed, and cardholder data is masked. `--filter PATH`
  (repeatable) prints only the selected fields as an object-shaped report with a
  `missing_filters` list so a typo is distinguishable from an absent field;
  `--format json` emits a machine-readable document; and `--unsafe` reveals raw
  values for local fault analysis.
- `diff`: compare two messages field by field, including nested EMV tags.
  `--filter` scopes the comparison and `--format json` is jq-friendly. Values are
  masked like `view` by default; `--unsafe` shows raw cardholder data.
- `redact`: mask the PAN, track, PIN, sensitive EMV tags, and a PAN embedded in a
  free-form private field, producing a sanitized document that is safe to share.
- `convert`: convert between a packed message and a JSON document, with the
  direction auto-detected from the input (`--to json|hex` to force it). Field 55
  is emitted per EMV tag so individual tags are editable, unknown tags are
  preserved across a round trip, and ambiguous documents (a path in both
  `fields` and `binary_fields`, or a parent path that also has nested children)
  are rejected with an explicit error.
- `validate`: report whether a message unpacks, plus unknown TLV tags and
  extension-field strategy. `--strict` adds best-effort, message-class-aware
  BASE I field checks. A failure names the field that could not be unpacked.
  Exit code is 0 for warnings only and 1 on an error.
- `sample`: list and export the bundled BASE I fixtures.
- A single optional `--config` JSON file selects the spec (`basei-starter`,
  `spec87ascii`, or a moov-io/iso8583 JSON spec) and overrides the
  extension-field catalog; a worked private-field overlay example is bundled.
- Safe-by-default cardholder-data handling: `view`, `diff`, and `redact` mask the
  PAN (BIN + last four), full track, PIN, the EMV tags that carry them, unknown
  TLV tags, and a PAN embedded in a private field, consistently across the
  describe, JSON, and filtered surfaces. Raw values are an explicit opt-in via
  `view --unsafe` / `diff --unsafe`; `convert` emits unmasked output so messages
  round-trip.
- Messages can be read from a file, from `-`, or from stdin, so the commands
  compose in pipes.
- Property-based tests (pgregory.net/rapid), Go fuzz targets, and end-to-end
  tests (shellspec) that drive the built binary, plus CI workflows for build,
  multi-platform unit tests, coverage (octocov), linting (golangci-lint via
  reviewdog), and e2e.

[Unreleased]: https://github.com/nao1215/iso8583tool/compare/v0.2.2...HEAD
[0.2.2]: https://github.com/nao1215/iso8583tool/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/nao1215/iso8583tool/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/nao1215/iso8583tool/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/nao1215/iso8583tool/releases/tag/v0.1.0

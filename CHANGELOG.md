# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- The full `view` describe output no longer over-masks a composite subfield whose
  local id collides with a top-level cardholder field. The top-level PAN/track/
  PIN masks (fields 2/34, 35/36/45, 52) are applied by dot-path, so a custom
  positional or bitmap composite subfield such as `48.2` or `127.2` keeps its real
  value instead of being masked by the field-2 PAN rule. The text view now masks
  the same paths as the JSON and diff views.
- Sensitive-data masking covers more free-form representations:
  - A labeled track (`TRACK2=4111…D2912…`, also `TRACK1=`/`TRACK=`) is masked in
    full, so the expiry, service code, and discretionary data trailing the PAN no
    longer leak — previously only the embedded PAN was masked.
  - PAN key labels are recognized in their common real-world spellings:
    underscore (`card_no`, `pan_no`, `account_no`), spaced (`card number`,
    `acct number`), hyphenated (`card-number`), long snake-case
    (`primary_account_number`), and camelCase (`primaryAccountNumber`).

- The `Extension Field Strategy` section now describes how the active spec
  actually models each field instead of the catalog's BASE I assumption: a
  positional composite reports `positional`, a bitmap composite `bitmap`, a
  BER-TLV composite `tlv`, and a plain field `opaque` (so the built-in field 127,
  modeled as a plain string, is no longer mislabeled `bitmap`).
- A bare custom `--spec PATH` no longer inherits the built-in BASE I extension
  catalog. A custom spec's fields are described by the spec itself — no BASE I
  field names (`ICC System Related Data`, `Additional Data - Private`), no
  spurious `tlv` note on a plain string field 55, and no `Extension Field
  Strategy` section at all — unless a catalog is supplied explicitly with
  `--config`.
- Setting a dot-path on a field the active spec models as a plain value (for
  example `48.1` or `127.1` under the built-in preset) now fails with a clear
  message — the field has no dot-path subfields — instead of leaking moov's
  internal `not a PathMarshaler` error.

- The full `view` describe output now keeps the parent path of a nested
  composite at any depth. A constructed BER-TLV leaf shows as `55.70.9F02` (and
  its header as `F55.70`), and a nested positional composite shows as `48.2.1`
  (header `F48.2`), instead of collapsing to the immediate parent (`70.9F02` /
  `2.1`). A known EMV tag is annotated even when nested — `55.70.8A` resolves to
  its ARC meaning and `55.70.9A` to its date — and those nested leaves now appear
  in the `decoded[]` list of `view --format json` and `validate --format json`.
- `view`, `validate`, `convert`, and `doctor` now tolerate a UTF-8 BOM
  (`EF BB BF`) at the start of a hex fixture, the way the JSON path already did.
  An editor that saves a hex capture with a BOM no longer breaks auto-detection
  (the file is read as hex, not mistaken for raw) or hex decoding. An explicit
  `--encoding raw` still treats the BOM bytes as part of the raw message.
- `redact` now defaults to `--encoding auto` like `view`, `validate`, `convert`,
  and `diff`. A raw `*.bin` capture (and an all-numeric raw ASCII message) is read
  correctly without `--encoding raw`; previously `redact` alone defaulted to
  `hex` and failed with `encoding/hex: invalid byte` on raw input even though its
  own help advertised `auto|hex|raw`.
- `convert` now rejects malformed and ambiguous document paths instead of
  silently mis-packing them. Field id `0` (the MTI) and `1` (the bitmap) can no
  longer be set through `fields` or `binary_fields`; a non-numeric or
  out-of-range id, leading/trailing whitespace, and empty path segments
  (`55..9F02`, `55.`) are reported with a clear message; and two spellings of the
  same field — `02` and `2`, or the BER-TLV tags `55.9f02` and `55.9F02` — are
  rejected as duplicate aliases instead of letting one silently overwrite the
  other.
- `convert` no longer drops nested BER-TLV data when a document sets both a
  top-level tag and a constructed one on the same field. Previously a message
  carrying `55.82` together with `55.70.9F02`/`55.70.9F36` packed only `55.82`;
  the flat-tag blob overwrote the whole composite. The flat tags are now merged
  into the composite's existing TLV stream, so every leaf path round-trips.

## [0.3.0] - 2026-06-05

This release fixes a broad set of correctness, ergonomics, and safety issues
found while exercising the CLI, and rounds out the built-in specs.

### Added

- The built-in presets now define the standard ISO 8583:1987 secondary-bitmap
  fields (66-128). A document that uses common high-numbered fields — for example
  95 (replacement amounts), 96 (message security code), 100 (receiving
  institution id), 102-104 (account ids / transaction description), or the
  reserved range 123-128 — now packs and round-trips instead of failing with
  "field N is not defined in the spec". Every field the default extension catalog
  documents (including 126 and 127) is now backed by a definition.

### Changed

- `view`, `validate`, `convert`, `diff`, and `redact` now default to
  `--encoding auto`, the same fit-based hex/raw detection `doctor` uses. A raw
  `*.bin` capture, and an all-numeric raw ASCII message that is byte-for-byte a
  valid hex string, are read correctly without `--encoding raw`. An explicit
  `--encoding hex|raw` still overrides the detection.

### Fixed

- A custom moov-io/iso8583 JSON spec passed with `--spec PATH` now loads the
  `Hex`, `Track1`, `Track3`, and `IndexTag` field types, both at the top level
  and inside composite subfields, and a composite `tag` block that omits `sort`
  now loads (defaulting to hex-tag order) instead of failing with a
  "no constructor for field type" or "unknown sort function" error. `IndexTag` is
  read as a positional string subfield (moov-io has no field type for that name).
- The `spec87bcd-starter` preset now matches its packed-BCD intent: field 55 is
  an EMV BER-TLV composite so `55.<tag>` packs and round-trips, the PIN (52) and
  MAC (64) fields are raw fixed-length binary, and variable-length fields encode
  their length prefix as BCD rather than ASCII.
- Zero-padded fixed-length fields show their canonical width consistently across
  every surface: the full `view` describe output and the `decoded[].value`
  entries (in `view --format json` and `validate --format json`) no longer
  collapse `000000` to `0`.
- `convert` tolerates a UTF-8 BOM (`EF BB BF`) at the start of a JSON document,
  both when auto-detecting the direction and with an explicit `--to hex`.
- `view --filter` and `diff --filter` match EMV tag paths case-insensitively
  (`55.9f02` = `55.9F02`), `diff --filter 0` selects the MTI like `view`, and
  `diff` reports a filter that matched nothing (text `No field matched filter:`,
  JSON `missing_filters`) so a typo is distinguishable from a real no-change
  result.
- `validate --strict` now checks advice and network-management messages that
  previously passed with only a STAN: an authorization/financial advice (`0120`,
  `0220`) is held to the same core requirements as its request, and every
  network-management message (`0810`, `0820`, `0830`, in addition to `0800`)
  requires the network-management code in field 70.
- `convert --output` reports the top-level ISO field count in its
  `packed N fields` summary, matching the `field_count` `doctor` reports, instead
  of counting every TLV subtag separately.
- `doctor` lists all presets tied at the best score rather than presenting the
  default as the single answer, and both `doctor` and `validate` call out a
  truncated or corrupt capture (too short to hold its MTI/bitmap) instead of
  steering the user to a custom spec or to `doctor` when neither can help.
- Constructed (nested) BER-TLV tags expand to their leaf dot-path (for example
  `55.70.9F02`) on unpack, so `convert`, `view --filter`, and `diff` resolve and
  compare the leaf tag instead of the parent tag's raw blob.
- Sensitive-data masking covers cases it previously missed, across `view`,
  `redact`, and `diff`:
  - A PAN or track carried in a binary (hex-encoded) field is masked, not only
    the text representation.
  - Sensitive EMV/TLV tags (`5A`, `56`, `57`, `99`, `9F1F`, `9F20`, and `9F6B`)
    are masked in any TLV container and at any nesting depth, known or unknown.
  - Free-form additional-data fields (for example 44 and 54) are content-scanned,
    and separator-grouped PANs (`4111 1111 ...`, `4111-1111-...`) are masked.
  - Field 34 (Extended PAN) is masked; field 20 (PAN Extended Country Code) is
    no longer masked.
  - The embedded-PAN scanner masks only Luhn-valid or PAN-key-labeled candidates,
    so a plain business identifier is left intact.
  - `view` reports a field's extension strategy as `tlv` when the active spec
    models it as a BER-TLV composite.
- Help requested on the success path (`--help`, `help <command>`,
  `<command> --help`, and the no-argument overview) prints to stdout; error-path
  usage still goes to stderr with a non-zero exit.
- The bundled `examples/spec87ascii/0800-network-echo` sample carries field 70,
  so it passes `validate --strict` under its intended `spec87ascii` preset.

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

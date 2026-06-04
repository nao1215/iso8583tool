# iso8583tool

[![Build](https://github.com/nao1215/iso8583tool/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/build.yml)
[![MultiPlatformUnitTest](https://github.com/nao1215/iso8583tool/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/unit_test.yml)
[![E2E](https://github.com/nao1215/iso8583tool/actions/workflows/e2e_test.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/e2e_test.yml)
[![reviewdog](https://github.com/nao1215/iso8583tool/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/reviewdog.yml)
[![Coverage](https://github.com/nao1215/iso8583tool/actions/workflows/coverage.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/coverage.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/iso8583tool.svg)](https://pkg.go.dev/github.com/nao1215/iso8583tool)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/iso8583tool)](https://goreportcard.com/report/github.com/nao1215/iso8583tool)
![GitHub](https://img.shields.io/github/license/nao1215/iso8583tool)

`iso8583tool` is a CLI for viewing, validating, and converting ISO 8583
messages. The default experience is tuned for BASE I, but the tool is not
BASE I-only: `--config` can switch to plain `spec87ascii` or any
[`moov-io/iso8583`](https://github.com/moov-io/iso8583) JSON spec.

![demo](./docs/demo.gif)

## Highlights

- BASE I starter workflow out of the box: built-in spec, extension catalog, and samples.
- Human-friendly decoding for MTI, response codes, amounts, dates, currencies, and EMV tags.
- Round-trippable JSON document format with per-tag Field 55 editing and unknown TLV preservation.
- Generic ISO 8583 support via `spec87ascii` or a custom moov JSON spec passed with `--config`.
- CLI ergonomics that work well in pipes: flags may appear before or after the target, JSON output is never colorized, and oversized input is rejected early.

## Install

```shell
go install github.com/nao1215/iso8583tool@latest
```

Or build from a clone:

```shell
make build   # produces ./iso8583tool
```

## Quick Start

```shell
iso8583tool sample
iso8583tool view examples/basei/0110-auth-response.hex
iso8583tool validate examples/basei/0100-auth-request-unknown-tlv.hex
iso8583tool convert examples/basei/0100-auth-request.hex
```

## Scope

The defaults are intentionally BASE I oriented:

- `basei-starter` is the built-in spec.
- Field 55 is modeled as EMV BER-TLV.
- Built-in samples live under [`examples/basei`](./examples/basei).
- The extension catalog explains how private fields such as 48, 55, 62, 63, 126, and 127 are treated.

The tool can also work with non-BASE-I message sets:

- `spec87ascii` switches to the plain ISO 8583:1987 ASCII spec.
- A config file may point at any moov JSON spec path, resolved relative to the config file.
- `view`, `convert`, and `validate` continue to work even when BASE I-specific overlays are not in use.

## Commands

```text
view       Unpack and inspect a message
convert    Convert between a packed message and a JSON document
validate   Check that a message unpacks and report issues
sample     List or export built-in BASE I samples
version    Print the version
```

A message can be read from a file, `-`, or stdin. Flags may appear before or
after the positional target. Use `--` before a dash-leading filename so it is
treated as a file, not as another flag.

## `view`

`view` unpacks a message and prints its fields. Known numeric and coded values
are translated to text, and PAN / track data are masked.

![view](./docs/demo-view.gif)

```shell
iso8583tool view examples/basei/0110-auth-response.hex
iso8583tool view examples/basei/0110-auth-response.hex --format json
iso8583tool view examples/basei/0110-auth-response.hex --filter 39 --filter 55.8A
cat examples/basei/0110-auth-response.hex | iso8583tool view -
```

## `convert`

`convert` auto-detects direction from the input:

- JSON document -> packed message
- packed message -> JSON document

Use `--to json|hex` to force a direction.

![convert](./docs/demo-convert.gif)

```shell
iso8583tool convert examples/basei/0100-auth-request.json
iso8583tool convert examples/basei/0100-auth-request.hex
iso8583tool sample 0100-auth-request --format hex | iso8583tool convert
iso8583tool convert examples/basei/0100-auth-request.json --output out.hex
```

## `validate`

`validate` checks whether a message unpacks and reports:

- decoded summary fields
- unknown TLV tags preserved for round-trip safety
- unpack failures with the field path that broke
- extension-field strategy for the active catalog

Exit code is `0` for success or warnings, and `1` for errors.

![validate](./docs/demo-validate.gif)

```shell
iso8583tool validate examples/basei/0100-auth-request-unknown-tlv.hex
iso8583tool validate --raw 01007220
iso8583tool validate examples/basei/0110-auth-response.hex --format json
```

## `sample`

`sample` lists and exports the built-in BASE I fixtures.

![sample](./docs/demo-sample.gif)

```shell
iso8583tool sample
iso8583tool sample 0100-auth-request
iso8583tool sample 0100-auth-request --format hex --output 0100.hex
```

## Message Document

`convert` and the JSON examples use this shape. `fields` holds text values,
`binary_fields` holds hex values, and keys are dot-paths. When a packed message
is unpacked to JSON, fixed-length values stay in their canonical padded form so
the document is easy to edit and pack back.

```json
{
  "mti": "0100",
  "fields": {
    "2": "4111111111111111",
    "3": "000000",
    "4": "000000005000",
    "49": "392"
  },
  "binary_fields": {
    "55.9F02": "000000005000",
    "55.9F36": "0034"
  }
}
```

> [!NOTE]
> The PAN `4111111111111111` used in the samples is a non-issued test number, not a real card.

## BASE I Extension Fields

`basei-starter` assigns a strategy to each private field so the path contract
can stay stable as a field is promoted from raw to structured.

| Field | Strategy   | Notes |
|-------|------------|-------|
| 48    | positional | raw string now; can grow into `48.1`, `48.2`, ... |
| 55    | tlv        | EMV BER-TLV; edit per tag, unknown tags preserved |
| 60    | positional | reserved national |
| 62    | positional | reserved private |
| 63    | opaque     | keep raw until the partner format is stable |
| 126   | opaque     | private late extensions |
| 127   | bitmap     | nested bitmap / subelement territory |

Field 55 is edited per tag. Known and unknown tags round-trip together:

![extension fields](./docs/demo-unknown-tlv.gif)

```shell
iso8583tool convert examples/basei/0100-auth-request-unknown-tlv.hex
iso8583tool convert examples/basei/0100-auth-request-unknown-tlv.hex | iso8583tool convert | iso8583tool view - --filter 55.DF8129
```

## Other Specs

BASE I is the default, not the limit. The repo also includes a minimal
`spec87ascii` example under [`examples/spec87ascii`](./examples/spec87ascii).

![spec87ascii](./docs/demo-spec87ascii.gif)

```shell
iso8583tool validate examples/spec87ascii/0800-network-echo.hex --config examples/spec87ascii.config.json
iso8583tool view examples/spec87ascii/0800-network-echo.hex --config examples/spec87ascii.config.json
iso8583tool convert examples/spec87ascii/0800-network-echo.hex --config examples/spec87ascii.config.json
```

The corresponding tape lives at [`docs/demo-spec87ascii.tape`](./docs/demo-spec87ascii.tape).

## Config

A config file is optional. It can select the message spec and, for BASE I-style
message sets, override the extension catalog.

Example: BASE I with catalog overrides:

```json
{
  "spec": "basei-starter",
  "extensions": [
    { "id": 55, "name": "ICC System Related Data", "strategy": "tlv", "preserve_unknown_tlv_tags": true },
    { "id": 63, "name": "Acme Settlement Blob", "strategy": "opaque" }
  ]
}
```

Example: plain ISO 8583:1987 ASCII:

```json
{
  "spec": "spec87ascii"
}
```

`spec` accepts:

- `basei-starter`
- `spec87ascii`
- a path to a moov JSON spec, relative to the config file

`strategy` accepts `opaque`, `tlv`, `positional`, or `bitmap`.

```shell
iso8583tool validate examples/basei/0110-auth-response.hex --config examples/iso8583tool.config.json
```

## CLI Notes

- `--color` is strict: use `auto`, `always`, or `never`. Unknown values fail instead of silently disabling color.
- `--no-color` is equivalent to `--color never`.
- JSON output is never colorized.
- A single input source is capped at 1 MiB, so obviously malformed or runaway input fails fast instead of being slurped whole.

## Development

```shell
make test       # unit tests with coverage
make test-e2e   # shellspec end-to-end tests against the built binary
make lint       # golangci-lint
make demo       # regenerate docs/*.gif from docs/*.tape
```

The command snippets in this README are covered by end-to-end tests under
[`spec/`](./spec).

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT. See [LICENSE](./LICENSE).

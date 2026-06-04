# iso8583tool

[![Build](https://github.com/nao1215/iso8583tool/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/build.yml)
[![MultiPlatformUnitTest](https://github.com/nao1215/iso8583tool/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/unit_test.yml)
[![E2E](https://github.com/nao1215/iso8583tool/actions/workflows/e2e_test.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/e2e_test.yml)
[![reviewdog](https://github.com/nao1215/iso8583tool/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/reviewdog.yml)
[![Coverage](https://github.com/nao1215/iso8583tool/actions/workflows/coverage.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/coverage.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/iso8583tool.svg)](https://pkg.go.dev/github.com/nao1215/iso8583tool)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/iso8583tool)](https://goreportcard.com/report/github.com/nao1215/iso8583tool)
![GitHub](https://img.shields.io/github/license/nao1215/iso8583tool)

A command-line tool for debugging and inspecting ISO 8583 payment messages.
Defaults to BASE I; other layouts are loaded from a `--config` spec.

![demo](./docs/demo.gif)

```shell
iso8583tool view examples/basei/0110-auth-response.hex
iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex
iso8583tool redact examples/basei/0100-auth-request.hex
```

Input comes from a file or stdin, and output is colored text on a terminal or
plain JSON, so commands pipe into `jq`, `grep`, and other tools:

```shell
iso8583tool view examples/basei/0110-auth-response.hex --format json | jq '.fields["39"]'
cat examples/basei/0110-auth-response.hex | iso8583tool view -
```

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

A message is read from a file, `-`, or stdin. Flags may come before or after the
positional argument. Use `--` before a dash-leading filename.

## Commands

```text
view       Unpack and inspect a message
diff       Compare two messages field by field
redact     Mask sensitive fields for safe sharing
convert    Convert between a packed message and a JSON document
validate   Check that a message unpacks and report issues
sample     List or export built-in BASE I samples
version    Print the version
```

## `view`

Unpacks a message and prints its fields. Coded values are decoded, and PAN and
track data are masked.

```shell
iso8583tool view examples/basei/0110-auth-response.hex
iso8583tool view examples/basei/0110-auth-response.hex --format json
iso8583tool view examples/basei/0110-auth-response.hex --filter 39 --filter 55.8A
cat examples/basei/0110-auth-response.hex | iso8583tool view -
```

JSON output works with `jq`:

```shell
iso8583tool view examples/basei/0110-auth-response.hex --format json | jq '.fields["39"]'
```

## `diff`

Compares two messages by field path, including nested EMV tags such as
`55.9F02`. Changes are marked added / removed / changed in a stable order.
Either side may be `-` for stdin.

```shell
iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex
iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex --filter 55
iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex --format json | jq '.changes[].path'
```

## `redact`

Masks the PAN, track data, PIN, and sensitive EMV tags (such as the application
cryptogram) so a message can be shared. The output is a sanitized document, not
a re-packable message.

```shell
iso8583tool redact examples/basei/0100-auth-request.hex
iso8583tool redact examples/basei/0100-auth-request.hex --format text
cat examples/basei/0100-auth-request.hex | iso8583tool redact -
```

## `convert`

Converts between a packed message and a JSON document. The direction is detected
from the input; use `--to json|hex` to force it.

```shell
iso8583tool convert examples/basei/0100-auth-request.json
iso8583tool convert examples/basei/0100-auth-request.hex
iso8583tool sample 0100-auth-request --format hex | iso8583tool convert
iso8583tool convert examples/basei/0100-auth-request.json --output out.hex
```

Unknown Field 55 tags are kept when converting, so a message round-trips to the
same bytes.

## `validate`

Reports whether a message unpacks, any unknown TLV tags, and the field path of
an unpack failure. Exit code is `0` for success or warnings, `1` for errors.

```shell
iso8583tool validate examples/basei/0100-auth-request-unknown-tlv.hex
iso8583tool validate --raw 01007220
iso8583tool validate examples/basei/0110-auth-response.hex --format json
```

## `sample`

Lists and exports the built-in BASE I fixtures.

```shell
iso8583tool sample
iso8583tool sample 0100-auth-request
iso8583tool sample 0100-auth-request --format hex --output 0100.hex
```

## Message document

`convert` and the JSON examples use this shape. `fields` holds text values,
`binary_fields` holds hex values, and keys are dot-paths. Fixed-length values
keep their padded form, so a document is easy to edit and pack back.

```json
{
  "mti": "0100",
  "fields": {
    "2": "4111111111111111",
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
> The PAN `4111111111111111` in the samples is a non-issued test number.

## BASE I defaults

The default spec is `basei-starter`: ASCII 1987 with Field 55 modeled as EMV
BER-TLV. Samples live under [`examples/basei`](./examples/basei). Each private
field has a strategy so the path stays stable as a field is promoted from raw to
structured:

| Field | Strategy   | Notes |
|-------|------------|-------|
| 48    | positional | raw string; can grow into `48.1`, `48.2`, ... |
| 55    | tlv        | EMV BER-TLV, edited per tag; unknown tags preserved |
| 62    | positional | reserved private |
| 63    | opaque     | raw until the partner format is stable |
| 127   | bitmap     | nested bitmap / subelement territory |

Field 55 is edited per tag, and unknown tags survive a round trip:

```shell
iso8583tool convert examples/basei/0100-auth-request-unknown-tlv.hex | iso8583tool convert | iso8583tool view - --filter 55.DF8129
```

## Other layouts

`--config` switches the spec. `spec87ascii` is the plain ISO 8583:1987 ASCII
spec; any [`moov-io/iso8583`](https://github.com/moov-io/iso8583) JSON spec works
too.

```shell
iso8583tool view examples/spec87ascii/0800-network-echo.hex --config examples/spec87ascii.config.json
```

A config selects the spec and, for BASE I-style sets, overrides the extension
catalog:

```json
{
  "spec": "basei-starter",
  "extensions": [
    { "id": 63, "name": "Acme Settlement Blob", "strategy": "opaque" }
  ]
}
```

`spec` is `basei-starter`, `spec87ascii`, or a path to a moov JSON spec relative
to the config file. `strategy` is `opaque`, `tlv`, `positional`, or `bitmap`.

## Fuzzing

Parsing untrusted input is fuzzed so malformed messages fail with an error
instead of crashing:

```shell
go test ./internal/service -run '^$' -fuzz=FuzzMessageToDocument
```

`FuzzDiffMessages` and `FuzzRedactMessage` are also available. Crashing inputs
are kept as regression seeds and replayed by `go test ./...`.

## Development

```shell
make test       # unit tests with coverage
make test-e2e   # shellspec end-to-end tests against the built binary
make lint       # golangci-lint
```

README command examples are covered by the end-to-end tests under
[`spec/`](./spec). See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT. See [LICENSE](./LICENSE).

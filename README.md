# iso8583tool

[![Build](https://github.com/nao1215/iso8583tool/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/build.yml)
[![MultiPlatformUnitTest](https://github.com/nao1215/iso8583tool/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/unit_test.yml)
[![E2E](https://github.com/nao1215/iso8583tool/actions/workflows/e2e_test.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/e2e_test.yml)
[![reviewdog](https://github.com/nao1215/iso8583tool/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/reviewdog.yml)
[![Coverage](https://github.com/nao1215/iso8583tool/actions/workflows/coverage.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/coverage.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/iso8583tool.svg)](https://pkg.go.dev/github.com/nao1215/iso8583tool)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/iso8583tool)](https://goreportcard.com/report/github.com/nao1215/iso8583tool)
![GitHub](https://img.shields.io/github/license/nao1215/iso8583tool)

A BASE I oriented ISO 8583 viewer, converter, and validator.

It uses [moov-io/iso8583](https://github.com/moov-io/iso8583) to pack and unpack
messages. The default spec is `basei-starter`: ASCII 1987 with Field 55 handled
as EMV BER-TLV. No setup is required; the built-in spec is used unless you pass
`--config`.

![demo](./docs/demo.gif)

## Install

```shell
go install github.com/nao1215/iso8583tool@latest
```

Or build from a clone:

```shell
make build   # produces ./iso8583tool
```

## Commands

```
view       Unpack and inspect a message
convert    Convert between a packed message and a JSON document
validate   Check that a message unpacks and report issues
sample     List or export built-in BASE I samples
version    Print the version
```

A message is read from a file, from `-`, or from stdin. Output is colored on a
terminal and plain when piped; use `--no-color` to force plain. Pass a spec or
extension catalog with `--config PATH`.

## view

Unpacks a message and prints its fields. Numeric codes (MTI, response code,
currency, amount, dates, EMV tags) are translated to text, and PAN and track
data are masked.

![view](./docs/demo-view.gif)

```shell
iso8583tool view examples/basei/0110-auth-response.hex
iso8583tool view examples/basei/0110-auth-response.hex --format json
iso8583tool view examples/basei/0110-auth-response.hex --filter 39 --filter 55.8A
cat examples/basei/0110-auth-response.hex | iso8583tool view -
```

## convert

Converts between a packed message and a JSON document. The direction is detected
from the input: a JSON document is packed to hex, a message is unpacked to a JSON
document. Use `--to json|hex` to force it.

![convert](./docs/demo-convert.gif)

```shell
iso8583tool convert examples/basei/0100-auth-request.json    # JSON -> hex
iso8583tool convert examples/basei/0100-auth-request.hex     # hex  -> JSON
iso8583tool sample 0100-auth-request --format hex | iso8583tool convert
iso8583tool convert examples/basei/0100-auth-request.json --output out.hex
```

## validate

Reports whether a message unpacks, plus unknown TLV tags and extension-field
strategy. Exit code is 0 when only warnings are present and 1 on an error. A
failure names the field that could not be unpacked.

![validate](./docs/demo-validate.gif)

```shell
iso8583tool validate examples/basei/0110-auth-response.hex
iso8583tool validate --raw 01007220        # broken: reports the failing field
```

## sample

Lists and exports the bundled BASE I fixtures.

![sample](./docs/demo-sample.gif)

```shell
iso8583tool sample
iso8583tool sample 0100-auth-request
iso8583tool sample 0100-auth-request --format hex --output 0100.hex
```

## Message document

`convert` and the JSON samples use this shape. `fields` holds text values,
`binary_fields` holds hex values, and keys are dot-paths.

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
> The PAN `4111111111111111` used in the samples is a non-issued test number, not a real card.

## Extension fields

`basei-starter` assigns each BASE I private field a strategy. The path-based
contract stays the same as a field is promoted from raw to structured.

| Field | Strategy   | Notes |
|-------|------------|-------|
| 48    | positional | raw string now; can grow into 48.1, 48.2 |
| 55    | tlv        | EMV BER-TLV; edit per tag, unknown tags preserved |
| 62    | positional | raw string |
| 63    | opaque     | raw string |
| 127   | bitmap     | metadata only |

Field 55 is edited per tag. Unpack a message to JSON, change or add a tag, and
pack it back. Tags that the spec does not know (here `DF8129`) survive the round
trip:

![extension fields](./docs/demo-unknown-tlv.gif)

```shell
iso8583tool convert examples/basei/0100-auth-request-unknown-tlv.hex > msg.json
# edit msg.json, e.g. set "55.9F02" to "000000010000" or add "55.DF01": "AABB"
iso8583tool convert msg.json --output msg.hex
```

## Config

A config is one JSON file that selects the spec and overrides the extension
catalog. It is optional.

![config](./docs/demo-config.gif)

```json
{
  "spec": "basei-starter",
  "extensions": [
    { "id": 55, "name": "ICC System Related Data", "strategy": "tlv", "preserve_unknown_tlv_tags": true },
    { "id": 63, "name": "Acme Settlement Blob", "strategy": "opaque" }
  ]
}
```

`spec` is `basei-starter`, `spec87ascii`, or a path to a moov-io/iso8583 JSON
spec (relative to the config file). `strategy` is `opaque`, `tlv`, `positional`,
or `bitmap`.

```shell
iso8583tool validate examples/basei/0110-auth-response.hex --config examples/iso8583tool.config.json
```

## Development

```shell
make test       # unit tests with coverage
make test-e2e   # shellspec end-to-end tests against the built binary
make lint       # golangci-lint
```

See [CONTRIBUTING.md](./CONTRIBUTING.md). End-to-end tests live under `spec/`.

## License

MIT. See [LICENSE](./LICENSE).

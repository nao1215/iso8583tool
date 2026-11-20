# iso8583tool

<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-1-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
[![Build](https://github.com/nao1215/iso8583tool/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/build.yml)
[![MultiPlatformUnitTest](https://github.com/nao1215/iso8583tool/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/unit_test.yml)
[![E2E](https://github.com/nao1215/iso8583tool/actions/workflows/e2e_test.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/e2e_test.yml)
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/iso8583tool/coverage.svg)
[![Gitleaks](https://github.com/nao1215/iso8583tool/actions/workflows/gitleaks.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/gitleaks.yml)
[![Trivy](https://github.com/nao1215/iso8583tool/actions/workflows/trivy.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/trivy.yml)
[![govulncheck](https://github.com/nao1215/iso8583tool/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/govulncheck.yml)
[![Gosec](https://github.com/nao1215/iso8583tool/actions/workflows/gosec.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/gosec.yml)
[![reviewdog](https://github.com/nao1215/iso8583tool/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/iso8583tool/actions/workflows/reviewdog.yml)
[![tested with atago](https://img.shields.io/badge/tested%20with-atago-7c3aed?logo=data:image/svg%2Bxml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI%2BPHBhdGggZmlsbD0iI2ZmZiIgZD0iTTMuNiA0LjIgMTEuOSAxMmwtOC4zIDcuOC0xLjktMi4yTDcuOSAxMiAxLjcgNi40eiIvPjxyZWN0IGZpbGw9IiNmZmYiIHg9IjEyLjYiIHk9IjE3LjIiIHdpZHRoPSI5LjciIGhlaWdodD0iMi44IiByeD0iMS40Ii8%2BPC9zdmc%2B&logoColor=white)](https://github.com/nao1215/atago)
[![measured with himorime](https://img.shields.io/badge/measured%20with-himorime-d9480f?logo=data:image/svg%2Bxml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI%2BPHBhdGggZmlsbD0ibm9uZSIgc3Ryb2tlPSIjZmZmIiBzdHJva2Utd2lkdGg9IjIuNCIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBkPSJNNC4yIDE4LjVBOSA5IDAgMSAxIDE5LjggMTguNSIvPjxwYXRoIGZpbGw9Im5vbmUiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIyLjQiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIgZD0iTTEyIDE0LjUgMTYuNSA5Ii8%2BPGNpcmNsZSBmaWxsPSIjZmZmIiBjeD0iMTIiIGN5PSIxNC41IiByPSIyLjIiLz48L3N2Zz4=&logoColor=white)](https://github.com/nao1215/himorime)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/iso8583tool.svg)](https://pkg.go.dev/github.com/nao1215/iso8583tool)
[![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/nao1215/iso8583tool/total)](https://github.com/nao1215/iso8583tool/releases)
[![License](https://img.shields.io/github/license/nao1215/iso8583tool)](./LICENSE)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/nao1215/iso8583tool/badge)](https://scorecard.dev/viewer/?uri=github.com/nao1215/iso8583tool)

A command-line tool for debugging and inspecting ISO 8583 payment messages.

![demo](./docs/demo.gif)

```shell
iso8583tool sample
iso8583tool view examples/basei/0110-auth-response.hex
iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex
iso8583tool redact examples/basei/0100-auth-request.hex
iso8583tool validate examples/basei/0100-auth-request-unknown-tlv.hex
iso8583tool convert examples/basei/0100-auth-request.hex
iso8583tool send 127.0.0.1:8583 examples/basei/0800-network-echo.hex
```

A message is read from a file, `-`, or stdin, and JSON output pipes into `jq`. Flags may come before or after the positional argument. Use `--` before a dash-leading filename.

```shell
iso8583tool view examples/basei/0110-auth-response.hex --format json | jq '.fields["39"]'
```

## Install

```shell
go install github.com/nao1215/iso8583tool@latest
```

Or build from a clone:

```shell
make build   # produces ./iso8583tool
```

### Verifying release integrity

Prebuilt archives for Linux, macOS, and Windows (`amd64` and `arm64`) are on [the release page](https://github.com/nao1215/iso8583tool/releases). Releases after v0.8.2 have `checksums.txt` signed with [cosign](https://github.com/sigstore/cosign) (keyless, as `checksums.txt.sigstore.json`), an SPDX SBOM (`*.sbom.json`) per archive, and SLSA build provenance attested via GitHub OIDC and attached as `multiple.intoto.jsonl`.

Verify the signed checksums, then your download:

```shell
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/nao1215/iso8583tool/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Verify the build provenance with the GitHub CLI:

```shell
gh attestation verify iso8583tool_<version>_<os>_<arch>.tar.gz --repo nao1215/iso8583tool
```

Or with [slsa-verifier](https://github.com/slsa-framework/slsa-verifier) and the `multiple.intoto.jsonl` asset:

```shell
slsa-verifier verify-artifact iso8583tool_<version>_<os>_<arch>.tar.gz \
  --provenance-path multiple.intoto.jsonl \
  --source-uri github.com/nao1215/iso8583tool \
  --source-tag v<version>
```

## Commands

```text
view       Unpack and inspect a message
diff       Compare two messages field by field
redact     Mask PAN, track, and EMV sensitive data
convert    Convert between a packed message and a JSON document
send       Send a message over TCP and decode the response
validate   Check that a message unpacks and report issues
doctor     Detect which built-in spec preset fits a message
specs      List the built-in spec presets
sample     List or export built-in BASE I samples
version    Print the version
```

Every command defaults to the `basei-starter` spec. If a capture does not decode, `iso8583tool doctor MESSAGE` finds the preset to pass with `--spec`.

## `view`

Unpacks a message, prints its fields and decodes coded values. Cardholder data is masked by default: the PAN, track, PIN, the EMV tags that carry them, and a PAN embedded in a free-form private field such as F63. `--unsafe` shows the raw values in `describe`, `json`, and filtered output.

![view](./docs/demo-view.gif)

```shell
iso8583tool view examples/basei/0110-auth-response.hex
iso8583tool view examples/basei/0110-auth-response.hex --format json
iso8583tool view examples/basei/0110-auth-response.hex --filter 39 --filter 55.8A
iso8583tool view examples/basei/0110-auth-response.hex --unsafe
cat examples/basei/0110-auth-response.hex | iso8583tool view -
```

```text
$ iso8583tool view examples/basei/0110-auth-response.hex
Summary: 0110 · Approved · JPY 5000 · STAN 123456 · TERMID01
...
F2   Primary Account Number...: 411111******1111
F39  Response Code............: 00  → Approved
F49  Transaction Currency Code: 392  → JPY (Japanese yen)
55.8A Authorisation Response Code: 3030  → Approved
...
```

With `--filter`, the JSON keeps the same object shape (`mti`, `fields`, `binary_fields`, `summary`, `decoded`) scoped to the matched paths, and always adds a `missing_filters` array, so a typo can be told apart from an absent field:

```shell
iso8583tool view examples/basei/0110-auth-response.hex --filter 39 --filter 90 --format json | jq '.missing_filters'
```

## `diff`

Compares two messages by field path, including nested EMV tags. Either side may be `-` for stdin. Values are compared raw but displayed masked as in `view` (PAN to BIN + last four; track, PIN, unknown TLV bytes, and a PAN embedded in a private field hidden), so the output can go into a ticket. `--unsafe` shows raw cardholder data.

![diff](./docs/demo-diff.gif)

```shell
iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex
iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex --filter 55
iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex --format json | jq '.changes[].path'
iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex --unsafe
```

```text
$ iso8583tool diff examples/basei/0100-auth-request.hex examples/basei/0110-auth-response.hex
MTI changed
- 0100
+ 0110

Field 12 changed
- 123456
+ 123457
...
```

## `redact`

Masks the PAN, track data, PIN, sensitive EMV tags, and a PAN embedded in a free-form private field. The output is a sanitized document that cannot be packed back into a message. `redact` has no raw mode; use `view --unsafe` or `diff --unsafe` to see raw values.

![redact](./docs/demo-redact.gif)

```shell
iso8583tool redact examples/basei/0100-auth-request.hex
iso8583tool redact examples/basei/0100-auth-request.hex --format text
cat examples/basei/0100-auth-request.hex | iso8583tool redact -
```

```text
$ iso8583tool redact examples/basei/0100-auth-request.hex
{
  "mti": "0100",
  "fields": {
    ...
    "2": "411111******1111",
    "35": "411111****************************",
    ...
  },
  "binary_fields": {
    ...
    "55.9F26": "****************"
  }
}
```

## `convert`

Converts between a packed message and a JSON document. The direction is detected from the input, or forced with `--to json|hex`. The message comes from a file argument, `-`/stdin, or an inline `--raw MESSAGE`.

```shell
iso8583tool convert examples/basei/0100-auth-request.json
iso8583tool convert examples/basei/0100-auth-request.hex
iso8583tool sample 0100-auth-request --format hex | iso8583tool convert
iso8583tool convert examples/basei/0100-auth-request.json --output out.hex
```

```text
$ iso8583tool convert examples/basei/0100-auth-request.hex
{
  "mti": "0100",
  "fields": {
    ...
    "2": "4111111111111111",
    ...
  },
  "binary_fields": {
    "55.9F02": "000000005000",
    ...
  }
}
```

Unknown Field 55 tags are preserved. `convert` does not mask anything, so the document round-trips with the PAN, track, and PIN in the clear; use [`redact`](#redact) before sharing it. `convert --help` says so, and a run prints a one-line reminder in the `--output` report, or on stderr when stdout is a terminal. Piped runs (`convert | convert`, scripts) get no extra output.

A document is rejected when a path is ambiguous: the same path in both `fields` and `binary_fields`, or a parent that also has nested children (for example `55` with `55.9F02`, or `48` with `48.1`). Packing it would depend on order and lose data.

## `send`

Sends one ISO 8583 message to a host over TCP and decodes the one response. It is a probe for a test endpoint or simulator, with no listener, session, retry, or sign-on logic.

![send](./docs/demo-send.gif)

The input is a JSON document (packed with the active spec) or a packed hex/raw message, read from a file, `-`, stdin, or `--raw`. `--raw` takes either a JSON document or a hex/raw string.

`--framing` selects the length header for both request and response:

- `2byte-binary` (default): a 2-byte big-endian length prefix.
- `4digit-ascii`: a 4-digit ASCII length header (`0094…`).
- `none`: no header. The request half-closes after the write, and the response is read until the peer closes or `--timeout`.

`--timeout` (default `5s`) bounds the connect and read. Output is masked like [`view`](#view) unless you pass `--unsafe`.

`--dry-run` packs and frames the request, prints the framing, the wire byte count and the decoded request, and exits without connecting. A malformed `HOST:PORT` is reported the same way with or without it. The framed bytes carry the PAN, track and PIN in the clear, so they are printed only with `--unsafe` (a `Framed bytes:` line, or `framed_hex` in `--format json`).

For CI, `--expect-mti VALUE` checks the response MTI and `--expect-field PATH=VALUE` (repeatable) checks a field, without `jq`. They compare against the unmasked values, so an expectation on a PAN matches its real value. A mismatch prints `expected … got …` on stderr and exits non-zero.

```shell
# Send a packed 0800 network echo and decode the 0810 reply (default framing).
iso8583tool send 127.0.0.1:8583 examples/basei/0800-network-echo.hex

# Pack a JSON document with the active spec, then send it with a 4-digit header.
iso8583tool send 127.0.0.1:8583 examples/basei/0100-auth-request.json --framing 4digit-ascii

# Send without a length header (read the reply until the peer closes or --timeout).
iso8583tool send 127.0.0.1:8583 examples/basei/0800-network-echo.hex --framing none

# Pass an inline message with --raw (JSON here; hex/raw also work).
iso8583tool send 127.0.0.1:8583 --raw '{"mti":"0800","fields":{"70":"301"}}'

# Verify the message packs and frames without sending it (no connection opened).
iso8583tool send 127.0.0.1:8583 examples/basei/0800-network-echo.hex --dry-run

# Assert the reply in CI: a non-zero exit on any mismatch, no jq required.
iso8583tool send 127.0.0.1:8583 examples/basei/0800-network-echo.hex \
  --expect-mti 0810 --expect-field 39=00

# Read the message from stdin and emit machine-readable JSON for jq.
iso8583tool sample 0800-network-echo --format hex \
  | iso8583tool send 127.0.0.1:8583 - --format json | jq '.response_view.mti'
```

```text
$ iso8583tool send 127.0.0.1:8583 examples/basei/0800-network-echo.hex
Sent to: 127.0.0.1:8583
Framing: 2byte-binary
Spec: basei-starter
Timeout: 5s
Sent bytes: 96  Received bytes: 108  RTT: 294µs

Request:
  Summary: 0800 · Echo test · STAN 654321 · TERMNET1
  0 = 0800  → Network management Request from Acquirer (ISO8583:1987)
  7 = 0604161616  → 06-04 16:16:16
  11 = 654321
  12 = 161616  → 16:16:16
  13 = 0604  → 06-04
  24 = 001
  41 = TERMNET1
  48 = HEARTBEAT=BASEI
  70 = 301  → Echo test

Response:
  Summary: 0810 · Approved · Echo test · STAN 654321 · TERMNET1
  0 = 0810  → Network management Response from Acquirer (ISO8583:1987)
  7 = 0604161616  → 06-04 16:16:16
  11 = 654321
  12 = 161617  → 16:16:17
  13 = 0604  → 06-04
  24 = 001
  39 = 00  → Approved
  41 = TERMNET1
  48 = HEARTBEAT=BASEI
  63 = ECHO=OK
  70 = 301  → Echo test
```

Like [`view`](#view), the output lists every present field, including ones with no decoded meaning such as F37/F38/F41/F42/F48/F63. `--format json` writes `remote_addr`, `framing`, `timeout`, `rtt_ms`, `sent_bytes`, `received_bytes`, the `request` / `response` (byte count, plus the raw wire `hex` only under `--unsafe`), and the decoded `request_view` / `response_view` in the shape of `view --format json`, all masked by default.

## `validate`

By default, `validate` reports whether a message unpacks, any unknown TLV tags, and the field path of an unpack failure. It does not check that the message is a complete BASE I transaction; a message with only a STAN can still unpack. The exit code is `0` for success or warnings and `1` for errors.

`--strict` adds heuristic checks per MTI for required and recommended fields: a `0110` response must carry a response code in field 39, an approved response should carry field 38, and a reversal needs field 90. It does not replace network certification.

```shell
iso8583tool validate examples/basei/0100-auth-request-unknown-tlv.hex
iso8583tool validate examples/basei/0110-auth-response.hex --format json
iso8583tool validate examples/basei/0110-auth-response.hex --strict
```

```text
$ iso8583tool validate examples/basei/0100-auth-request-unknown-tlv.hex
Validation: ok
Spec: basei-starter
MTI: 0100  → Authorization Request from Acquirer (ISO8583:1987)
...
Issues:
- [warning] 55.DF8129: unknown TLV tag preserved for round-trip safety
```

A broken message reports the error and exits non-zero (`--raw` passes an inline message):

```shell
$ iso8583tool validate --raw 01007220
Validation: failed
Hint: the message did not unpack under basei-starter; run `iso8583tool doctor` to detect the right spec
...
- [error] ...
$ echo $?
1
```

The hint appears whenever a message fails to unpack, because the usual cause is the wrong spec. See [`doctor`](#doctor).

## `doctor`

ISO 8583 does not fix a wire encoding: the bitmap can be ASCII, packed BCD, or binary, and private fields differ per network, so a capture decodes only under the spec that produced it. `doctor` tries every built-in preset and ranks them by an exact byte-length round trip, a clean unpack, a valid MTI, and the number of decoded fields. It detects hex text or raw bytes on its own, so a raw `.bin` capture needs no flags; `--encoding hex|raw` overrides the guess.

```shell
iso8583tool doctor examples/basei/0110-auth-response.hex
iso8583tool doctor message.bin
iso8583tool doctor examples/basei/0110-auth-response.hex --format json | jq .recommended
```

```text
$ iso8583tool doctor examples/basei/0110-auth-response.hex
Doctor: inspected 216 bytes
Recommended: --spec basei-starter

Candidates:
- basei-starter      recommended  MTI 0110, 16 fields, exact byte-length round trip
- spec87ascii        no           cannot unpack field 55: invalid ASCII char ...
- spec87bcd-starter  no           cannot unpack field 45: invalid syntax ...

Confirm with: iso8583tool view examples/basei/0110-auth-response.hex --spec basei-starter
```

`doctor` exits non-zero when no built-in preset can unpack the message, which usually means it needs a custom `moov-io/iso8583` JSON spec. When several presets fit, confirm with `view`. `basei-starter` and `spec87ascii` differ only in Field 55 (EMV BER-TLV or plain ASCII), so for a message without it `doctor` names both; pick `basei-starter` if Field 55 carries EMV/ICC data.

## `specs`

Lists the built-in presets that `--spec` accepts. A `moov-io/iso8583` JSON spec path also works as `--spec`.

```shell
iso8583tool specs
iso8583tool specs --format json | jq -r '.[].name'
```

```text
$ iso8583tool specs
Built-in spec presets (use with --spec NAME):

- basei-starter (default)
  BASE I Starter ASCII
  encoding: ASCII fields, ASCII-hex bitmap, field 55 as EMV BER-TLV
  Default. BASE I authorization/financial traffic with EMV ICC data in field 55.
...
```

## `sample`

Lists and exports the built-in BASE I fixtures.

```shell
iso8583tool sample
iso8583tool sample 0100-auth-request
iso8583tool sample 0100-auth-request --format hex --output 0100.hex
```

```text
$ iso8583tool sample
Available samples:
- 0100-auth-request: EMV authorization request with BASE I style private fields 48 and 62
- 0110-auth-response: EMV authorization response with issuer data in field 55 and opaque field 63
...
```

## Message document

`convert` and the JSON examples use this shape. `fields` holds text values, `binary_fields` holds hex values, and keys are dot-paths. Fixed-length values keep their padding, so a document can be edited and packed back.

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

The default spec `basei-starter` is ASCII 1987 with Field 55 as EMV BER-TLV. Samples are under [`examples/basei`](./examples/basei). `view` reports how the active spec models each private field:

| Field | Shown as | Notes |
|-------|----------|-------|
| 48    | opaque   | plain value; suggest a positional overlay once the segment layout is fixed |
| 55    | tlv      | EMV BER-TLV, edited per tag; unknown tags preserved |
| 62    | opaque   | reserved private |
| 63    | opaque   | raw until the partner format is stable |
| 127   | opaque   | plain value; nested-bitmap territory for a custom spec |

A dot-path such as `48.1` works only when a custom `--spec` models field 48 as a composite. Field 55 is edited per tag, and unknown tags survive a round trip:

```shell
iso8583tool convert examples/basei/0100-auth-request-unknown-tlv.hex | iso8583tool convert | iso8583tool view - --filter 55.DF8129
```

## Other layouts

`spec87ascii` is the plain ISO 8583:1987 ASCII spec. `spec87bcd-starter` is a raw-binary starter with packed-BCD MTI and common numeric fields and a binary bitmap, for quiz-style fixtures such as `kanmu/gocon-2022-spring/message.bin`. Any [`moov-io/iso8583`](https://github.com/moov-io/iso8583) JSON spec works too. [`iso8583tool specs`](#specs) lists the presets and [`iso8583tool doctor`](#doctor) detects which one a capture uses.

```shell
iso8583tool view examples/spec87ascii/0800-network-echo.hex --spec spec87ascii
iso8583tool view message.bin --encoding raw --spec spec87bcd-starter
```

`--config` takes extension overlays and default bundles. The `extensions` list replaces the built-in catalog, so list every private field you want annotated:

```json
{
  "spec": "basei-starter",
  "extensions": [
    { "id": 63, "name": "Acme Settlement Blob", "strategy": "opaque" }
  ]
}
```

`spec` in the config file sets a default that the `--spec` flag overrides. `strategy` is `opaque`, `tlv`, `positional`, or `bitmap`. Without the `extensions` key the built-in catalog stays; `"extensions": []` turns it off, so no private fields are annotated.

[`examples/basei-overlay-config.json`](./examples/basei-overlay-config.json) relabels the private fields F48/F55/F62/F63/F127 for a fictional acquirer:

```shell
iso8583tool view examples/basei/0110-auth-response.hex --config examples/basei-overlay-config.json
```

## Testing

| Technique | Bug class caught here |
|---|---|
| [Unit tests and atago E2E](./e2e/atago) | Wrong output, exit codes and README examples of each command |
| [Property-based tests](./internal/service/pbt_test.go) | Round trips that drift, redaction that leaks a cardholder value |
| [Metamorphic relations](./internal/service/metamorphic_test.go) | Masking that is not idempotent, presets that read a message differently |
| [Differential tests against mofax/iso8583](./internal/service/differential_test.go) | Wrong field lengths in the 1987 table, packing that another implementation reads differently |
| [Fuzzing](./internal/service/fuzz_test.go) | Crashes and PAN leaks on malformed messages |
| [Mutation testing](./.github/workflows/mutation.yml) | Conditions the tests run but do not check (82.35% efficacy on 2026-11-19) |
| [Paired CI benchmarks](./bench/himorime.yaml) | Commands that got slower or use more memory |

Commands and details: [docs/testing.md](./docs/testing.md).

## Development

```shell
make test       # unit tests with coverage
make e2e        # atago end-to-end tests against a freshly built binary
make lint       # golangci-lint
```

The atago tests under [`e2e/atago/`](./e2e/atago) run the README command examples. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## Contributors ✨

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/en/reference/emoji-key/)):

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://debimate.jp/"><img src="https://avatars.githubusercontent.com/u/22737008?v=4?s=75" width="75px;" alt="CHIKAMATSU Naohiro"/><br /><sub><b>CHIKAMATSU Naohiro</b></sub></a><br /><a href="https://github.com/nao1215/iso8583tool/commits?author=nao1215" title="Code">💻</a> <a href="https://github.com/nao1215/iso8583tool/commits?author=nao1215" title="Documentation">📖</a></td>
    </tr>
  </tbody>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!

## License

MIT. See [LICENSE](./LICENSE).

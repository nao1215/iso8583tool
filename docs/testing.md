# Testing

This page lists how each technique in the README's Testing table runs and where its tests live.

## Unit tests and atago E2E

`make test` runs the unit tests with coverage. `make e2e` builds iso8583tool and the mock TCP server in `e2e/mock` and runs the atago scenarios in [`e2e/atago/`](../e2e/atago), which also execute the README command examples. `make coverage` combines both into `coverage.out`; CI publishes the result through octocov.

## Property-based tests

[`internal/service/pbt_test.go`](../internal/service/pbt_test.go) uses [pgregory.net/rapid](https://pgregory.net/rapid) to generate BASE I documents and checks that convert is a fixed point (pack, unpack and pack again give the same bytes), that redact never prints a cardholder value, that diff is symmetric and reflexive, that the masks keep the length, and that strict validation is monotonic. The normal `go test` run draws 100 cases per property; run more with:

```shell
go test ./internal/service -run TestPBT -rapid.checks=20000
```

A failure prints the seed and writes the failing case under `testdata/rapid/`, which `-rapid.failfile` replays.

## Metamorphic relations

[`internal/service/metamorphic_test.go`](../internal/service/metamorphic_test.go) checks relations between two runs instead of a fixed answer: redacting a redacted message changes nothing, masking is idempotent, hex encoding and decoding are inverses, a message converted between presets reads back the same, and the describe view never contains a raw control byte whatever the field holds.

## Differential tests

[`internal/service/differential_test.go`](../internal/service/differential_test.go) compares the ISO 8583:1987 ASCII preset with [github.com/mofax/iso8583](https://github.com/mofax/iso8583), an independent implementation with its own field table, used only by the tests. It checks three things:

- Every field from 2 to 128 has the same length type and maximum length in both tables. The preset's fields 66 to 128 are written by hand in `internal/basei/spec.go`, so this is what checks them against the standard.
- A generated message packs to the same bytes in both implementations.
- Each implementation reads back what the other one wrote.

Two differences are expected and handled in the test, each with its reason in a comment: the preset accepts up to 99 characters in field 44 where ISO 8583:1987 allows 25, and the preset always sends a 128-bit bitmap without setting bit 1, where ISO 8583:1987 sends the secondary bitmap only when bit 1 is set. The test translates the bitmap between the two forms and compares every other byte exactly.

```shell
go test ./internal/service -run TestDifferential -rapid.checks=5000
```

## Fuzzing

The fuzz targets parse untrusted input and check that a malformed message fails with an error instead of a crash, and that convert, redact and view keep their properties on whatever parses:

- `internal/service`: `FuzzMessageToDocument`, `FuzzDiffMessages`, `FuzzRedactMessage`, `FuzzConvertRoundTrip`, `FuzzValidateNoPanic`, `FuzzViewNeverLeaksPAN`
- `internal/messageio`: `FuzzParseDocument`, `FuzzCanonicalPath`, `FuzzDecodeInput`
- `internal/render`: `FuzzSanitizeControl`

`go test ./...` replays the seeds and the saved regressions under `testdata/fuzz/`. The nightly `fuzz.yml` workflow runs each target for a few minutes. Run one locally with:

```shell
go test ./internal/service -run '^$' -fuzz '^FuzzMessageToDocument$' -fuzztime 1m
```

## Mutation testing

[gremlins](https://github.com/go-gremlins/gremlins) v0.6.0 changes one expression at a time (a `<` to `<=`, a `==` to `!=`, a `+` to `-`) and runs the tests against each mutant. A mutant that no test notices (lived) points at a condition the tests execute but do not check. `mutation.yml` mutates every package weekly and, on a pull request, only the lines the pull request changes. Run it locally with:

```shell
gremlins unleash --workers 2 --timeout-coefficient 10 .
```

| Date | Killed | Lived | Not covered | Timed out | Efficacy (killed / (killed + lived)) |
|---|---|---|---|---|---|
| 2026-11-19 | 644 | 138 | 108 | 9 | 82.35% |

The efficacy is a record, not a gate. Some lived mutants are equivalent (a boundary change on a length that is never equal to the limit), so the number is not expected to reach 100%.

## Paired CI benchmarks

[`bench/himorime.yaml`](../bench/himorime.yaml) measures each command with [himorime](https://github.com/nao1215/himorime). On a pull request, `bench.yml` builds the base and the head and measures them alternately on the same runner, so a regression is judged against the base and not against a stored number. Run it locally with `make bench`, or `make bench-compare` against main.

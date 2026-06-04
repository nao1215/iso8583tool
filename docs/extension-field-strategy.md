# Extension Field Strategy

`iso8583tool` treats BASE I as two layers:

1. A generic ISO 8583 message spec handled by `moov-io/iso8583`
2. A BASE I private overlay that explains how selected fields should be viewed and edited

The starter scaffold intentionally keeps these layers separate because the hardest part of BASE I is not the top-level bitmap. It is the partner-specific meaning of extension fields such as `48`, `55`, `60`, `62`, `63`, `126`, and `127`.

## Default Strategy

The workspace starts with `basei-starter`, which uses an ASCII 1987 message spec and upgrades field `55` into an EMV TLV composite. It also adds `specs/extensions.json` as metadata for private fields.

Each extension field is assigned one of four strategies:

- `opaque`: keep the field editable as one raw value until its private structure is trustworthy
- `positional`: treat the field as numbered subfields once the byte layout is stable
- `tlv`: treat the field as TLV or BER-TLV and preserve unknown tags for round-trip safety
- `bitmap`: reserve a dedicated nested-bitmap model for switches that pack private subelements under a secondary bitmap

## Editing Model

The write-side JSON format is intentionally path-oriented:

```json
{
  "mti": "0100",
  "fields": {
    "2": "4111111111111111",
    "48.1": "issuer-private-subfield",
    "55.9F02": "000000001000"
  }
}
```

That gives one stable editing contract for both flat and nested fields:

- top-level fields use `48`, `55`, `127`
- positional overlays use `48.1`, `48.2`, `127.25.1`
- TLV overlays use `55.9F02`, `55.9F36`

The scaffold does not force every extension field into a composite immediately. That would make the starter project look complete while encoding the wrong private grammar.

## Promotion Rules

Promote a field from `opaque` only when at least one of these is true:

- the network spec defines stable positional segments
- the field is clearly TLV-based and unknown tags must survive re-pack
- the switch uses a nested bitmap or subelement registry that is stable across message classes

When that happens:

1. Add or update the external JSON spec pointed to by `message_spec`
2. Change the field entry in `specs/extensions.json`
3. Keep the path-based command interface unchanged so callers do not need a new contract

## Why This Split Matters

This structure keeps the first version honest:

- `view` and `validate` can work immediately against a standard ISO8583 baseline
- `write` already uses a path-based document format that scales to nested fields, including `55.9F02`
- BASE I private knowledge lives in one catalog instead of being smeared across CLI code

Once the exact BASE I profile is available, the scaffold can move from "viewer/writer shell" to "network-accurate parser/editor" without rewriting the command surface.

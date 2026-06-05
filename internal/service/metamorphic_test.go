package service

import (
	"reflect"
	"testing"

	"github.com/moov-io/iso8583"
	"pgregory.net/rapid"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/render"
)

// TestPBTRedactIsIdempotent is a metamorphic property: redacting an
// already-redacted message changes nothing. Masking is a closure operation, so
// a "safe to share" document stays byte-for-byte identical when redacted again.
func TestPBTRedactIsIdempotent(t *testing.T) {
	t.Parallel()
	spec := pbtSpec(t)
	rapid.Check(t, func(t *rapid.T) {
		raw, err := WriteMessage(genDocument(t), spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack: %v", err)
		}
		once, _, err := RedactMessage(spec.MessageSpec, raw.Raw)
		if err != nil {
			t.Fatalf("redact: %v", err)
		}
		// Re-pack the redacted document and redact again; the document must be a
		// fixed point of redaction.
		repacked, err := WriteMessage(once, spec.MessageSpec)
		if err != nil {
			return // a masked value (e.g. "****") may not re-pack under a numeric field; skip
		}
		twice, _, err := RedactMessage(spec.MessageSpec, repacked.Raw)
		if err != nil {
			t.Fatalf("redact twice: %v", err)
		}
		if !reflect.DeepEqual(once, twice) {
			t.Fatalf("redaction is not idempotent:\n once=%#v\n twice=%#v", once, twice)
		}
	})
}

// TestPBTMaskEmbeddedSensitiveIsIdempotent asserts the free-form content scanner
// is a fixed point: a value with its PANs/tracks already masked is unchanged by
// a second pass.
func TestPBTMaskEmbeddedSensitiveIsIdempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		// Mix labels, digits, and separators so candidates appear.
		v := rapid.StringMatching(`(PAN=|card_no=|TRACK2=|ref )?[0-9 -]{0,40}`).Draw(t, "v")
		once := maskEmbeddedSensitive(v)
		twice := maskEmbeddedSensitive(once)
		if once != twice {
			t.Fatalf("maskEmbeddedSensitive not idempotent: %q -> %q -> %q", v, once, twice)
		}
	})
}

// TestPBTEncodingRoundTrip asserts EncodeOutput/DecodeInput are inverses for hex:
// decoding the hex encoding of any bytes returns the original bytes.
func TestPBTEncodingRoundTrip(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.SliceOfN(rapid.Byte(), 1, 64).Draw(t, "raw")
		hexed, err := messageio.EncodeOutput(raw, "hex")
		if err != nil {
			t.Fatalf("EncodeOutput: %v", err)
		}
		back, err := messageio.DecodeInput(hexed, "hex")
		if err != nil {
			t.Fatalf("DecodeInput on our own hex output failed: %v (%q)", err, hexed)
		}
		if !reflect.DeepEqual(raw, back) {
			t.Fatalf("hex round-trip changed the bytes: %x -> %s -> %x", raw, hexed, back)
		}
	})
}

// TestPBTSanitizeNeverLeaksControlBytesInView is a cross-surface metamorphic
// property: regardless of the bytes a describe view renders, the output carries
// no raw ESC/control byte (other than the color escapes, which are disabled
// here with a no-color palette).
func TestPBTViewDescribeHasNoRawControlBytes(t *testing.T) {
	t.Parallel()
	spec := pbtSpec(t)
	rapid.Check(t, func(t *rapid.T) {
		raw, err := WriteMessage(genDocument(t), spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack: %v", err)
		}
		res, err := ViewMessage(raw.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false), false)
		if err != nil {
			t.Fatalf("view: %v", err)
		}
		for i := 0; i < len(res.Body); i++ {
			b := res.Body[i]
			if b == '\n' || b == '\t' {
				continue
			}
			if b < 0x20 || b == 0x7f {
				t.Fatalf("describe body contains a raw control byte 0x%02x at %d", b, i)
			}
		}
	})
}

// TestPBTCrossPresetRoundTrip is a metamorphic test across presets: a digits-only
// document round-trips through both the ASCII and the packed-BCD preset. The wire
// bytes differ (ASCII vs packed BCD), but the canonical document must come back
// identical from each.
func TestPBTCrossPresetRoundTrip(t *testing.T) {
	t.Parallel()
	presets := map[string]*iso8583.MessageSpec{
		"ascii": basei.StarterMessageSpec(),
		"bcd":   basei.Spec87BCDStarter(),
	}
	rapid.Check(t, func(t *rapid.T) {
		// A small all-numeric network-management document both presets accept.
		doc := messageio.Document{MTI: "0800", Fields: map[string]string{
			"11": digits(t, 6, "stan"),
			"70": digits(t, 3, "nmic"),
			"74": digits(t, 10, "credits"),
		}}
		for name, spec := range presets {
			raw, err := WriteMessage(doc, spec)
			if err != nil {
				t.Fatalf("%s pack: %v", name, err)
			}
			back, err := MessageToDocument(spec, raw.Raw)
			if err != nil {
				t.Fatalf("%s unpack: %v", name, err)
			}
			for k, v := range doc.Fields {
				if back.Fields[k] != v {
					t.Fatalf("%s round-trip changed field %s: %q -> %q", name, k, v, back.Fields[k])
				}
			}
		}
	})
}

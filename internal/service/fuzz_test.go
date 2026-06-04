package service

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/render"
)

// fuzzSeeds returns a few valid packed messages to anchor the corpus so the
// fuzzer mutates around real structure (bitmaps, LLVAR lengths, BER-TLV).
func fuzzSeeds(f *testing.F) [][]byte {
	f.Helper()
	spec := basei.StarterMessageSpec()
	var seeds [][]byte
	for _, sample := range basei.StarterSamples() {
		res, err := WriteMessage(sample.Document, spec)
		if err != nil {
			f.Fatalf("seed pack %s: %v", sample.Name, err)
		}
		seeds = append(seeds, res.Raw)
	}
	return seeds
}

// FuzzMessageToDocument ensures unpacking arbitrary bytes (malformed bitmaps,
// truncated LLVAR lengths, broken BER-TLV) never panics.
func FuzzMessageToDocument(f *testing.F) {
	spec := basei.StarterMessageSpec()
	for _, seed := range fuzzSeeds(f) {
		f.Add(seed)
	}
	f.Add([]byte{})
	f.Add([]byte("0100"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = MessageToDocument(spec, data)
	})
}

// FuzzRedactMessage ensures redaction of arbitrary input never panics.
func FuzzRedactMessage(f *testing.F) {
	spec := basei.StarterMessageSpec()
	for _, seed := range fuzzSeeds(f) {
		f.Add(seed)
	}

	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _, _ = RedactMessage(spec, data)
	})
}

// FuzzDiffMessages ensures diffing two arbitrary inputs never panics.
func FuzzDiffMessages(f *testing.F) {
	spec := basei.StarterMessageSpec()
	seeds := fuzzSeeds(f)
	if len(seeds) >= 2 {
		f.Add(seeds[0], seeds[1])
	}

	f.Fuzz(func(_ *testing.T, before, after []byte) {
		_, _ = DiffMessages(spec, before, after, nil, false)
	})
}

// FuzzConvertRoundTrip checks the convert contract: any message that unpacks
// must re-pack, and the canonical document must be a fixed point. A failure
// here is a real round-trip bug (convert hex->json->hex would not be stable).
func FuzzConvertRoundTrip(f *testing.F) {
	spec := basei.StarterMessageSpec()
	for _, seed := range fuzzSeeds(f) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		doc1, err := MessageToDocument(spec, data)
		if err != nil {
			return // not a well-formed message; nothing to round-trip
		}
		raw1, err := WriteMessage(doc1, spec)
		if err != nil {
			// A few spec87 fields (e.g. F52 PIN, which uses BytesToASCIIHex +
			// Hex.Fixed) cannot be re-packed even by moov itself. convert
			// surfaces that as a clear error rather than crashing or corrupting,
			// which is acceptable; the invariant we assert is the fixed point
			// for everything that does re-pack.
			return
		}
		doc2, err := MessageToDocument(spec, raw1.Raw)
		if err != nil {
			// moov accepts some malformed field-55 BER-TLV on unpack (a bare tag
			// byte, or an empty-value tag) that, once re-encoded canonically,
			// its own strict unpack rejects. That upstream lenient-in/strict-out
			// asymmetry is surfaced as a clear error, never a crash; well-formed
			// messages round-trip (see TestPBTConvertRoundTripIsFixedPoint).
			return
		}
		// Compare everything except field 55. Field 55 is BER-TLV: moov stores
		// unusual unknown tags flat on unpack (a constructed tag, bit 6 set) but
		// re-nests them on the next unpack, so the structure is not a fixed point
		// for those adversarial tags. Real EMV uses primitive DFxx/9Fxx tags,
		// whose full round-trip is asserted by TestPBTConvertRoundTripIsFixedPoint.
		if !reflect.DeepEqual(without55(doc1), without55(doc2)) {
			t.Fatalf("canonical document (excluding field 55) is not a fixed point:\n doc1=%#v\n doc2=%#v", doc1, doc2)
		}
	})
}

// FuzzValidateNoPanic ensures strict validation of arbitrary input never panics.
func FuzzValidateNoPanic(f *testing.F) {
	spec := basei.StarterMessageSpec()
	for _, seed := range fuzzSeeds(f) {
		f.Add(seed)
	}

	f.Fuzz(func(_ *testing.T, data []byte) {
		_ = ValidateMessage(data, spec, "basei-starter", basei.DefaultExtensionCatalog(), true)
	})
}

// without55 returns a copy of the document with its field-55 binary entries
// removed, so a comparison can ignore BER-TLV round-trip quirks.
func without55(d messageio.Document) messageio.Document {
	out := messageio.Document{MTI: d.MTI, Fields: d.Fields}
	if d.BinaryFields != nil {
		bf := map[string]string{}
		for k, v := range d.BinaryFields {
			if strings.HasPrefix(k, "55.") {
				continue
			}
			bf[k] = v
		}
		if len(bf) > 0 {
			out.BinaryFields = bf
		}
	}
	return out
}

// FuzzViewNeverLeaksPAN asserts the describe view masks each cardholder field
// exactly as the canonical mask functions do. Checking equality against the
// expected mask (rather than substring presence of the raw value) avoids false
// positives when a value coincidentally equals its own masked form.
func FuzzViewNeverLeaksPAN(f *testing.F) {
	spec := basei.StarterMessageSpec()
	for _, seed := range fuzzSeeds(f) {
		f.Add(seed)
	}
	pal := render.NewPalette(false)
	maskFor := map[string]func(string) string{
		"2": maskPAN, "20": maskPAN, "35": maskTrack, "36": maskTrack, "45": maskTrack,
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		canon, err := MessageToDocument(spec, data)
		if err != nil {
			return
		}
		res, err := ViewMessage(data, spec, basei.DefaultExtensionCatalog(), "describe", nil, pal, false)
		if err != nil {
			return
		}
		for id, mask := range maskFor {
			raw, ok := canon.Fields[id]
			if !ok || strings.ContainsAny(raw, "\r\n") {
				continue // a value with newlines cannot be matched on one describe line
			}
			shown, found := describeFieldValue(res.Body, id)
			if found && shown != mask(raw) {
				t.Fatalf("describe masking mismatch for field %s: shown %q, expected %q (raw %q)", id, shown, mask(raw), raw)
			}
		}
	})
}

// describeFieldValue returns the value rendered for top-level field id in moov's
// describe output (the text after ": " on the "F<id> ..." line).
func describeFieldValue(body, id string) (string, bool) {
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "F"+id {
			continue
		}
		if idx := strings.Index(line, ": "); idx >= 0 {
			return line[idx+2:], true
		}
	}
	return "", false
}

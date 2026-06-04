package service

import (
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
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
		_, _ = DiffMessages(spec, before, after, nil)
	})
}

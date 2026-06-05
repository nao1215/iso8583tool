package messageio

import (
	"strings"
	"testing"
)

// FuzzParseDocument ensures parsing arbitrary JSON never panics, and that any
// document it accepts has only canonical, well-formed paths — the validation
// invariant the rest of the pipeline relies on.
func FuzzParseDocument(f *testing.F) {
	for _, seed := range []string{
		`{"mti":"0100","fields":{"2":"4111111111111111"}}`,
		`{"mti":"0100","binary_fields":{"55.9F02":"000000005000"}}`,
		`{"mti":"0100","fields":{"0":"x"}}`,
		`{"mti":"0100","fields":{"02":"a","2":"b"}}`,
		`{"mti":"0100","binary_fields":{"55..9F02":"00"}}`,
		`{"mti":"0100","fields":{" 2":"x"}}`,
		`not json`,
		`{}`,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := ParseDocument(data)
		if err != nil {
			return // a rejected document carries no invariant to check
		}
		// An accepted document must have a non-empty MTI and only canonical paths.
		if doc.MTI == "" {
			t.Fatalf("accepted a document with an empty MTI: %q", data)
		}
		for _, group := range []map[string]string{doc.Fields, doc.BinaryFields} {
			for path := range group {
				canon, cerr := canonicalPath(path)
				if cerr != nil {
					t.Fatalf("accepted document has an invalid path %q: %v", path, cerr)
				}
				_ = canon
				if path != strings.TrimSpace(path) {
					t.Fatalf("accepted document has a whitespace path %q", path)
				}
			}
		}
	})
}

// FuzzCanonicalPath ensures path canonicalization never panics and is idempotent:
// canonicalizing a canonical path yields the same value.
func FuzzCanonicalPath(f *testing.F) {
	for _, seed := range []string{"2", "02", "55.9F02", "55.70.8a", "48.2.1", "0", "1", "A", "-2", "129", " 2", "55..9F02"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, path string) {
		canon, err := canonicalPath(path)
		if err != nil {
			return
		}
		again, err := canonicalPath(canon)
		if err != nil {
			t.Fatalf("a canonical path %q was rejected on re-canonicalization: %v", canon, err)
		}
		if again != canon {
			t.Fatalf("canonicalPath is not idempotent: %q -> %q -> %q", path, canon, again)
		}
	})
}

// FuzzDecodeInput ensures decoding arbitrary bytes under each encoding never
// panics.
func FuzzDecodeInput(f *testing.F) {
	for _, seed := range []string{"3031", "  30 31  ", "zzzz", "", "\xef\xbb\xbf3031"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		for _, enc := range []string{"hex", "raw", ""} {
			_, _ = DecodeInput(data, enc)
		}
	})
}

package service

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/messagespec"
)

func pbtSpec(t *testing.T) *messagespec.Spec {
	t.Helper()
	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	return spec
}

func digits(t *rapid.T, n int, label string) string {
	return rapid.StringMatching("[0-9]{"+strconv.Itoa(n)+"}").Draw(t, label)
}

func hexs(t *rapid.T, nbytes int, label string) string {
	return rapid.StringMatching("[0-9A-F]{"+strconv.Itoa(2*nbytes)+"}").Draw(t, label)
}

func cloneDoc(d messageio.Document) messageio.Document {
	n := messageio.Document{MTI: d.MTI}
	if d.Fields != nil {
		n.Fields = make(map[string]string, len(d.Fields))
		for k, v := range d.Fields {
			n.Fields[k] = v
		}
	}
	if d.BinaryFields != nil {
		n.BinaryFields = make(map[string]string, len(d.BinaryFields))
		for k, v := range d.BinaryFields {
			n.BinaryFields[k] = v
		}
	}
	return n
}

// genDocument draws a packable BASE I document: a bundled sample with its
// amount, STAN, PAN, and a few EMV/unknown tags replaced by random valid
// values. Building on a real sample keeps it packable while still exercising
// length, value, and unknown-tag variation.
func genDocument(t *rapid.T) messageio.Document {
	base := rapid.SampledFrom(basei.StarterSamples()).Draw(t, "sample")
	doc := cloneDoc(base.Document)

	if _, ok := doc.Fields["4"]; ok {
		doc.Fields["4"] = digits(t, 12, "f4")
	}
	if _, ok := doc.Fields["11"]; ok {
		doc.Fields["11"] = digits(t, 6, "f11")
	}
	if _, ok := doc.Fields["2"]; ok {
		doc.Fields["2"] = digits(t, rapid.IntRange(12, 19).Draw(t, "panlen"), "pan")
	}
	if doc.BinaryFields != nil {
		if _, ok := doc.BinaryFields["55.9F02"]; ok {
			doc.BinaryFields["55.9F02"] = hexs(t, 6, "f9f02")
		}
	}

	// Optionally append unknown DFxx tags (second byte < 0x80 keeps the BER-TLV
	// tag two bytes long, so it is well-formed but unknown to the spec).
	for i := 0; i < rapid.IntRange(0, 2).Draw(t, "nUnknown"); i++ {
		if doc.BinaryFields == nil {
			doc.BinaryFields = map[string]string{}
		}
		tag := fmt.Sprintf("DF%02X", rapid.IntRange(1, 0x7F).Draw(t, "unktag"+strconv.Itoa(i)))
		doc.BinaryFields["55."+tag] = hexs(t, rapid.IntRange(1, 8).Draw(t, "unklen"+strconv.Itoa(i)), "unkval"+strconv.Itoa(i))
	}
	return doc
}

// TestPBTConvertRoundTripIsFixedPoint asserts that once a document has been
// packed and unpacked, further hex<->json conversions are stable: the canonical
// document and its packed bytes never drift. This is the core convert contract.
func TestPBTConvertRoundTripIsFixedPoint(t *testing.T) {
	t.Parallel()
	spec := pbtSpec(t)
	rapid.Check(t, func(t *rapid.T) {
		doc := genDocument(t)

		raw0, err := WriteMessage(doc, spec.MessageSpec)
		if err != nil {
			t.Fatalf("WriteMessage on a valid document failed: %v\n%#v", err, doc)
		}

		doc1, err := MessageToDocument(spec.MessageSpec, raw0.Raw)
		if err != nil {
			t.Fatalf("MessageToDocument on freshly packed bytes failed: %v", err)
		}
		raw1, err := WriteMessage(doc1, spec.MessageSpec)
		if err != nil {
			t.Fatalf("re-pack of a canonical document failed: %v\n%#v", err, doc1)
		}
		doc2, err := MessageToDocument(spec.MessageSpec, raw1.Raw)
		if err != nil {
			t.Fatalf("re-unpack failed: %v", err)
		}

		if !reflect.DeepEqual(doc1, doc2) {
			t.Fatalf("canonical document is not a fixed point:\n doc1=%#v\n doc2=%#v", doc1, doc2)
		}
		raw2, err := WriteMessage(doc2, spec.MessageSpec)
		if err != nil {
			t.Fatalf("re-pack of doc2 failed: %v", err)
		}
		if string(raw1.Raw) != string(raw2.Raw) {
			t.Fatalf("packing the canonical document is not deterministic")
		}
	})
}

// sensitivePaths returns the paths that redact/view treat as cardholder data,
// for the given canonical document.
func sensitivePaths(doc messageio.Document) []string {
	paths := []string{"2", "34", "35", "36", "45", "52"}
	for _, tag := range cardholderEMVTags {
		paths = append(paths, "55."+tag)
	}
	for k := range doc.BinaryFields {
		if strings.HasPrefix(k, "55.") {
			tag := strings.TrimPrefix(k, "55.")
			if !knownEMVTag(tag) {
				paths = append(paths, k) // unknown TLV tag
			}
		}
	}
	return paths
}

func knownEMVTag(tag string) bool {
	for _, known := range []string{
		"5F2A", "71", "72", "82", "84", "8A", "91", "95", "9A", "9C",
		"9F02", "9F03", "9F09", "9F10", "9F1A", "9F1E", "9F26", "9F27",
		"9F33", "9F34", "9F35", "9F36", "9F37", "9F41",
	} {
		if known == tag {
			return true
		}
	}
	return false
}

// TestPBTRedactNeverLeaksSensitiveValues asserts redact's "safe to share"
// guarantee: every sensitive field is masked length-preservingly and its
// original bytes never survive in the redacted document.
func TestPBTRedactNeverLeaksSensitiveValues(t *testing.T) {
	t.Parallel()
	spec := pbtSpec(t)
	rapid.Check(t, func(t *rapid.T) {
		doc := genDocument(t)
		raw, err := WriteMessage(doc, spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack: %v", err)
		}

		canon, err := MessageToDocument(spec.MessageSpec, raw.Raw)
		if err != nil {
			t.Fatalf("MessageToDocument: %v", err)
		}
		red, _, err := RedactMessage(spec.MessageSpec, raw.Raw)
		if err != nil {
			t.Fatalf("RedactMessage: %v", err)
		}

		flatCanon := FlattenDocument(canon)
		flatRed := FlattenDocument(red)

		for _, path := range sensitivePaths(canon) {
			original, ok := flatCanon[path]
			if !ok || original == "" {
				continue
			}
			redacted, ok := flatRed[path]
			if !ok {
				t.Fatalf("sensitive path %q vanished from the redacted document", path)
			}
			if len(redacted) != len(original) {
				t.Fatalf("mask at %q changed length: %d -> %d", path, len(original), len(redacted))
			}
			if strings.Contains(redacted, original) {
				t.Fatalf("redact leaked %q: %q still contains %q", path, redacted, original)
			}
		}
	})
}

// TestPBTDiffIsSymmetric asserts diff(a,b) and diff(b,a) are mirror images:
// same paths, kinds swapped (added<->removed), and before/after swapped.
func TestPBTDiffIsSymmetric(t *testing.T) {
	t.Parallel()
	spec := pbtSpec(t)
	rapid.Check(t, func(t *rapid.T) {
		rawA, err := WriteMessage(genDocument(t), spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack a: %v", err)
		}
		rawB, err := WriteMessage(genDocument(t), spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack b: %v", err)
		}

		fwd, err := DiffMessages(spec.MessageSpec, rawA.Raw, rawB.Raw, nil, true)
		if err != nil {
			t.Fatalf("diff fwd: %v", err)
		}
		rev, err := DiffMessages(spec.MessageSpec, rawB.Raw, rawA.Raw, nil, true)
		if err != nil {
			t.Fatalf("diff rev: %v", err)
		}

		revByPath := indexDiff(rev.Changes)
		if len(fwd.Changes) != len(rev.Changes) {
			t.Fatalf("diff is asymmetric: %d vs %d changes", len(fwd.Changes), len(rev.Changes))
		}
		for _, f := range fwd.Changes {
			r, ok := revByPath[f.Path]
			if !ok {
				t.Fatalf("path %q present forward but not reverse", f.Path)
			}
			switch f.Kind {
			case DiffChanged:
				if r.Kind != DiffChanged || r.Before != f.After || r.After != f.Before {
					t.Fatalf("changed %q not mirrored: fwd=%#v rev=%#v", f.Path, f, r)
				}
			case DiffAdded:
				if r.Kind != DiffRemoved || r.Before != f.After {
					t.Fatalf("added %q not mirrored to removed: fwd=%#v rev=%#v", f.Path, f, r)
				}
			case DiffRemoved:
				if r.Kind != DiffAdded || r.After != f.Before {
					t.Fatalf("removed %q not mirrored to added: fwd=%#v rev=%#v", f.Path, f, r)
				}
			}
		}
	})
}

// TestPBTDiffReflexive asserts a message never differs from itself.
func TestPBTDiffReflexive(t *testing.T) {
	t.Parallel()
	spec := pbtSpec(t)
	rapid.Check(t, func(t *rapid.T) {
		raw, err := WriteMessage(genDocument(t), spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack: %v", err)
		}
		for _, unsafe := range []bool{false, true} {
			res, err := DiffMessages(spec.MessageSpec, raw.Raw, raw.Raw, nil, unsafe)
			if err != nil {
				t.Fatalf("diff: %v", err)
			}
			if len(res.Changes) != 0 {
				t.Fatalf("a message differs from itself (unsafe=%v): %#v", unsafe, res.Changes)
			}
		}
	})
}

// TestPBTDiffDetectsSingleAmountChange is a metamorphic test: changing only the
// amount (field 4, non-sensitive) must yield exactly one change at path "4".
func TestPBTDiffDetectsSingleAmountChange(t *testing.T) {
	t.Parallel()
	spec := pbtSpec(t)
	rapid.Check(t, func(t *rapid.T) {
		doc := genDocument(t)
		if _, ok := doc.Fields["4"]; !ok {
			return // sample without an amount field
		}
		before := doc.Fields["4"]
		after := digits(t, 12, "newAmount")
		if after == before {
			return
		}
		mutated := cloneDoc(doc)
		mutated.Fields["4"] = after

		rawA, err := WriteMessage(doc, spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack a: %v", err)
		}
		rawB, err := WriteMessage(mutated, spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack b: %v", err)
		}

		res, err := DiffMessages(spec.MessageSpec, rawA.Raw, rawB.Raw, nil, false)
		if err != nil {
			t.Fatalf("diff: %v", err)
		}
		if len(res.Changes) != 1 || res.Changes[0].Path != "4" || res.Changes[0].Kind != DiffChanged {
			t.Fatalf("expected exactly one change at field 4, got %#v", res.Changes)
		}
		if res.Changes[0].Before != before || res.Changes[0].After != after {
			t.Fatalf("amount change not reported verbatim: %#v", res.Changes[0])
		}
	})
}

// TestPBTDiffMaskedNeverLeaks asserts the default (masked) diff never prints a
// full PAN or full track even when those fields change.
func TestPBTDiffMaskedNeverLeaks(t *testing.T) {
	t.Parallel()
	spec := pbtSpec(t)
	rapid.Check(t, func(t *rapid.T) {
		doc := genDocument(t)
		if _, ok := doc.Fields["2"]; !ok {
			return
		}
		mutated := cloneDoc(doc)
		panA := doc.Fields["2"]
		panB := digits(t, rapid.IntRange(12, 19).Draw(t, "panB"), "panBval")
		mutated.Fields["2"] = panB
		if _, ok := mutated.Fields["35"]; ok {
			mutated.Fields["35"] = panB + "D29122011234567"
		}

		rawA, err := WriteMessage(doc, spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack a: %v", err)
		}
		rawB, err := WriteMessage(mutated, spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack b: %v", err)
		}

		res, err := DiffMessages(spec.MessageSpec, rawA.Raw, rawB.Raw, nil, false)
		if err != nil {
			t.Fatalf("diff: %v", err)
		}
		for _, c := range res.Changes {
			for _, pan := range []string{panA, panB} {
				if len(pan) >= 12 && (strings.Contains(c.Before, pan) || strings.Contains(c.After, pan)) {
					t.Fatalf("masked diff leaked a full PAN at %q: %#v", c.Path, c)
				}
			}
		}
	})
}

// TestPBTComparePathsTotalOrder checks the diff path comparator is a strict
// total order (antisymmetric and transitive) over a realistic path vocabulary.
func TestPBTComparePathsTotalOrder(t *testing.T) {
	t.Parallel()
	vocab := []string{"mti", "2", "3", "4", "10", "11", "55", "62", "127",
		"55.82", "55.9F02", "55.9F36", "55.DF01", "127.25", "127.25.1", "0"}
	gen := rapid.SampledFrom(vocab)
	rapid.Check(t, func(t *rapid.T) {
		a := gen.Draw(t, "a")
		b := gen.Draw(t, "b")
		c := gen.Draw(t, "c")

		if comparePaths(a, a) != 0 {
			t.Fatalf("comparePaths(%q,%q) != 0", a, a)
		}
		if sign(comparePaths(a, b)) != -sign(comparePaths(b, a)) {
			t.Fatalf("not antisymmetric: cmp(%q,%q)=%d cmp(%q,%q)=%d", a, b, comparePaths(a, b), b, a, comparePaths(b, a))
		}
		if comparePaths(a, b) < 0 && comparePaths(b, c) < 0 && comparePaths(a, c) >= 0 {
			t.Fatalf("not transitive: %q < %q < %q but not %q < %q", a, b, c, a, c)
		}
	})
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

// TestStrictValidationIsMonotonic is a metamorphic test: removing a hard-required
// field from a strict-valid sample must make it strict-invalid and name the
// field. Fields with OR-alternates (the PAN sources) are excluded.
func TestStrictValidationIsMonotonic(t *testing.T) {
	t.Parallel()
	spec := pbtSpec(t)

	cases := []struct {
		doc      messageio.Document
		required []string
	}{
		{basei.AuthRequest(), []string{"3", "4", "7", "11"}},
		{basei.AuthResponse(), []string{"11", "39"}},
		{basei.FinancialRequest(), []string{"3", "4", "7", "11"}},
		{basei.FinancialResponse(), []string{"11", "39"}},
		{basei.NetworkEchoRequest(), []string{"11", "70"}},
		{basei.NetworkEchoResponse(), []string{"11", "39"}},
	}

	for _, tc := range cases {
		base := ValidateMessageDoc(t, spec, tc.doc)
		if base.HasErrors() {
			t.Fatalf("sample %s should be strict-valid, issues=%#v", tc.doc.MTI, base.Issues)
		}
		for _, fieldID := range tc.required {
			mutated := cloneDoc(tc.doc)
			delete(mutated.Fields, fieldID)
			report := ValidateMessageDoc(t, spec, mutated)
			if !report.HasErrors() {
				t.Fatalf("%s without field %s should be strict-invalid", tc.doc.MTI, fieldID)
			}
			if !hasIssue(report, "error", fieldID) {
				t.Fatalf("%s missing field %s should flag an error at %s, issues=%#v", tc.doc.MTI, fieldID, fieldID, report.Issues)
			}
		}
	}
}

// ValidateMessageDoc packs a document and runs strict validation on it.
func ValidateMessageDoc(t *testing.T, spec *messagespec.Spec, doc messageio.Document) ValidationReport {
	t.Helper()
	raw, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack %s: %v", doc.MTI, err)
	}
	return ValidateMessage(raw.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), true)
}

// TestPBTMaskFunctionsPreserveLength asserts the masking primitives never change
// a value's length and never panic, for arbitrary inputs.
func TestPBTMaskFunctionsPreserveLength(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		v := rapid.String().Draw(t, "v")
		for name, fn := range map[string]func(string) string{
			"maskPAN":   maskPAN,
			"maskTrack": maskTrack,
			"maskAll":   maskAll,
		} {
			if got := fn(v); len(got) != len(v) {
				t.Fatalf("%s changed length for %q: %d -> %d", name, v, len(v), len(got))
			}
		}
	})
}

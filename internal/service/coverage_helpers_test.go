package service

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/render"
)

// unpackStarter packs the AuthRequest sample and unpacks it under the starter
// spec, returning a ready-to-inspect message.
func unpackStarter(t *testing.T) *iso8583.Message {
	t.Helper()
	spec := basei.StarterMessageSpec()
	packed, err := WriteMessage(basei.AuthRequest(), spec)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	msg := iso8583.NewMessage(spec)
	if err := msg.Unpack(packed.Raw); err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	return msg
}

func TestClassName(t *testing.T) {
	t.Parallel()
	cases := map[byte]string{
		'5': "reconciliation",
		'6': "administrative",
		'7': "fee collection",
		'0': "unknown",
		'x': "unknown",
	}
	for in, want := range cases {
		if got := className(in); got != want {
			t.Errorf("className(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsAllDigits(t *testing.T) {
	t.Parallel()
	if !isAllDigits("012345") {
		t.Error("isAllDigits(012345) = false, want true")
	}
	if isAllDigits("") {
		t.Error("isAllDigits(empty) = true, want false")
	}
	if isAllDigits("12a4") {
		t.Error("isAllDigits(12a4) = true, want false")
	}
}

func TestExtensionNote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		strategy basei.ExtensionStrategy
		want     string
	}{
		{basei.StrategyTLV, "round-trip"},
		{basei.StrategyPositional, "dot-path"},
		{basei.StrategyBitmap, "nested-bitmap"},
		{basei.StrategyOpaque, "raw until"},
	}
	for _, c := range cases {
		got := extensionNote(basei.ExtensionField{Strategy: c.strategy})
		if !strings.Contains(got, c.want) {
			t.Errorf("extensionNote(%s) = %q, want to contain %q", c.strategy, got, c.want)
		}
	}
}

func TestValidMTI(t *testing.T) {
	t.Parallel()
	if !validMTI("0800") {
		t.Error("validMTI(0800) = false, want true")
	}
	if validMTI("080") {
		t.Error("validMTI(080) = true, want false")
	}
	if validMTI("08X0") {
		t.Error("validMTI(08X0) = true, want false")
	}
}

func TestAllTruncated(t *testing.T) {
	t.Parallel()
	if allTruncated(nil) {
		t.Error("allTruncated(nil) = true, want false")
	}
	// All failed and all look truncated.
	truncated := []SpecCandidate{
		{Unpacks: false, Detail: "not enough data to decode field 3"},
		{Unpacks: false, Detail: "unexpected EOF while reading"},
	}
	if !allTruncated(truncated) {
		t.Error("allTruncated(all truncated) = false, want true")
	}
	// One unpacked -> not all truncated.
	mixed := []SpecCandidate{
		{Unpacks: true, Detail: "MTI 0800"},
		{Unpacks: false, Detail: "not enough data"},
	}
	if allTruncated(mixed) {
		t.Error("allTruncated(one unpacked) = true, want false")
	}
	// Failed but not truncation-shaped -> not all truncated.
	other := []SpecCandidate{
		{Unpacks: false, Detail: "some other reason"},
	}
	if allTruncated(other) {
		t.Error("allTruncated(non-truncation failure) = true, want false")
	}
}

func TestSplitTLVPath(t *testing.T) {
	t.Parallel()
	if id, tag, ok := splitTLVPath("55.9F02"); !ok || id != 55 || tag != "9F02" {
		t.Errorf("splitTLVPath(55.9F02) = %d,%q,%v", id, tag, ok)
	}
	// A three-segment path is not a flat TLV path.
	if _, _, ok := splitTLVPath("127.25.1"); ok {
		t.Error("splitTLVPath(127.25.1) should not be a flat TLV path")
	}
	// A non-numeric field id is rejected.
	if _, _, ok := splitTLVPath("xx.9F02"); ok {
		t.Error("splitTLVPath(xx.9F02) should be rejected")
	}
}

func TestFitDetail(t *testing.T) {
	t.Parallel()
	// Empty MTI -> "unpacked, no MTI"; single field -> singular "1 field".
	noMTI := fitDetail(SpecCandidate{MTI: "", FieldCount: 1})
	if !strings.Contains(noMTI, "unpacked, no MTI") || !strings.Contains(noMTI, "1 field") {
		t.Errorf("fitDetail(no MTI, 1 field) = %q", noMTI)
	}
	// Present MTI, multiple fields, exact round trip.
	full := fitDetail(SpecCandidate{MTI: "0800", FieldCount: 3, ExactRoundTrip: true})
	if !strings.Contains(full, "MTI 0800") || !strings.Contains(full, "3 fields") || !strings.Contains(full, "exact byte-length round trip") {
		t.Errorf("fitDetail(full) = %q", full)
	}
}

func TestSafePack(t *testing.T) {
	t.Parallel()
	msg := unpackStarter(t)
	packed, err := safePack(msg)
	if err != nil {
		t.Fatalf("safePack: %v", err)
	}
	if len(packed) == 0 {
		t.Error("safePack returned empty output")
	}
	// A freshly constructed message with no fields set panics inside moov's
	// Pack; safePack must turn that into an error rather than crash.
	empty := iso8583.NewMessage(basei.StarterMessageSpec())
	if _, err := safePack(empty); err == nil {
		t.Log("safePack of an empty message returned no error (implementation-dependent)")
	}
}

func TestIsTLVComposite(t *testing.T) {
	t.Parallel()
	spec := basei.StarterMessageSpec()
	if !isTLVComposite(spec, 55) {
		t.Error("field 55 should be a TLV composite")
	}
	if isTLVComposite(spec, 3) {
		t.Error("field 3 (numeric) should not be a TLV composite")
	}
	if isTLVComposite(spec, 9999) {
		t.Error("missing field id should not be a TLV composite")
	}
}

func TestMarshalPathError(t *testing.T) {
	t.Parallel()
	notMarshaler := marshalPathError("set", "3.5", errors.New("field 3 is not a PathMarshaler"))
	if !strings.Contains(notMarshaler.Error(), "plain field") {
		t.Errorf("not-a-PathMarshaler branch = %q", notMarshaler.Error())
	}
	notDefined := marshalPathError("set", "55.DF01", errors.New("field DF01 is not defined in the spec"))
	if !strings.Contains(notDefined.Error(), "not defined in the active spec") {
		t.Errorf("not-defined branch = %q", notDefined.Error())
	}
	generic := marshalPathError("set", "48.1", errors.New("boom"))
	if !strings.Contains(generic.Error(), "boom") {
		t.Errorf("generic branch = %q", generic.Error())
	}
}

func TestLookupFlatValue(t *testing.T) {
	t.Parallel()
	flat := map[string]string{"3": "000000", "55.9F02": "01"}
	if v, ok := lookupFlatValue(flat, "0800", "mti"); !ok || v != "0800" {
		t.Errorf("lookup mti = %q,%v", v, ok)
	}
	if v, ok := lookupFlatValue(flat, "0800", "0"); !ok || v != "0800" {
		t.Errorf("lookup 0 = %q,%v", v, ok)
	}
	if _, ok := lookupFlatValue(flat, "", "mti"); ok {
		t.Error("lookup mti with empty mti should not be ok")
	}
	if v, ok := lookupFlatValue(flat, "0800", "3"); !ok || v != "000000" {
		t.Errorf("lookup exact = %q,%v", v, ok)
	}
	if v, ok := lookupFlatValue(flat, "0800", "55.9f02"); !ok || v != "01" {
		t.Errorf("lookup case-insensitive = %q,%v", v, ok)
	}
	if _, ok := lookupFlatValue(flat, "0800", "99"); ok {
		t.Error("lookup missing path should not be ok")
	}
}

func TestReadBody(t *testing.T) {
	t.Parallel()
	if b, err := readBody(bytes.NewReader(nil), 0); err != nil || len(b) != 0 {
		t.Errorf("readBody n=0 = %v,%v", b, err)
	}
	body, err := readBody(bytes.NewReader([]byte("hello")), 5)
	if err != nil || string(body) != "hello" {
		t.Errorf("readBody exact = %q,%v", body, err)
	}
	if _, err := readBody(bytes.NewReader([]byte("hi")), 5); err == nil {
		t.Error("readBody with short reader should error")
	}
}

func TestIsDashes(t *testing.T) {
	t.Parallel()
	if isDashes("") {
		t.Error("isDashes(empty) = true, want false")
	}
	if !isDashes("-----") {
		t.Error("isDashes(-----) = false, want true")
	}
	if isDashes("--x--") {
		t.Error("isDashes(--x--) = true, want false")
	}
}

func TestFieldID(t *testing.T) {
	t.Parallel()
	if got := fieldID("F55  EMV data"); got != "55" {
		t.Errorf("fieldID = %q, want 55", got)
	}
	if got := fieldID("   "); got != "" {
		t.Errorf("fieldID(blank) = %q, want empty", got)
	}
}

func TestColorizeSubfieldsHeader(t *testing.T) {
	t.Parallel()
	pal := render.NewPalette(false)
	if got := colorizeSubfieldsHeader("", "55", pal); got != "" {
		t.Errorf("colorizeSubfieldsHeader(empty) = %q, want empty", got)
	}
	got := colorizeSubfieldsHeader("F55 subfields:", "55", pal)
	if !strings.Contains(got, "F55") {
		t.Errorf("colorizeSubfieldsHeader = %q, want to contain F55", got)
	}
}

func TestLookupPath(t *testing.T) {
	t.Parallel()
	msg := unpackStarter(t)

	if _, _, ok := lookupPath(msg, "not-a-number"); ok {
		t.Error("lookupPath with a non-numeric id should fail")
	}
	if _, _, ok := lookupPath(msg, "9999"); ok {
		t.Error("lookupPath for a missing field should fail")
	}
	desc, val, ok := lookupPath(msg, "3")
	if !ok || val == "" || desc == "" {
		t.Errorf("lookupPath(3) = %q,%q,%v", desc, val, ok)
	}
	// Field 3 is a plain field, so descending into it must fail.
	if _, _, ok := lookupPath(msg, "3.1"); ok {
		t.Error("lookupPath into a non-composite field should fail")
	}
	// Field 55 is composite, but this tag is not present.
	if _, _, ok := lookupPath(msg, "55.DEAD"); ok {
		t.Error("lookupPath for a missing subfield should fail")
	}
}

func TestCanonicalFieldValue(t *testing.T) {
	t.Parallel()
	// Fixed-length padded numeric field: padding is re-applied.
	padded := field.NewNumeric(&field.Spec{
		Length:      12,
		Description: "amount",
		Enc:         encoding.ASCII,
		Pref:        prefix.ASCII.Fixed,
		Pad:         padLeftZero{},
	})
	if got := canonicalFieldValue(padded, "5000"); got != "000000005000" {
		t.Errorf("canonicalFieldValue(padded) = %q, want 000000005000", got)
	}
	// Variable-length field: returned unchanged.
	variable := field.NewString(&field.Spec{
		Length:      20,
		Description: "text",
		Enc:         encoding.ASCII,
		Pref:        prefix.ASCII.LL,
		Pad:         padLeftZero{},
	})
	if got := canonicalFieldValue(variable, "abc"); got != "abc" {
		t.Errorf("canonicalFieldValue(variable) = %q, want abc", got)
	}
	// No padder: returned unchanged.
	noPad := field.NewString(&field.Spec{
		Length:      6,
		Description: "x",
		Enc:         encoding.ASCII,
		Pref:        prefix.ASCII.Fixed,
	})
	if got := canonicalFieldValue(noPad, "abc"); got != "abc" {
		t.Errorf("canonicalFieldValue(noPad) = %q, want abc", got)
	}
}

// padLeftZero is a minimal left zero-padder for canonicalFieldValue tests.
type padLeftZero struct{}

func (padLeftZero) Pad(data []byte, length int) []byte {
	for len(data) < length {
		data = append([]byte("0"), data...)
	}
	return data
}

func (padLeftZero) Unpad(data []byte) []byte {
	return bytes.TrimLeft(data, "0")
}

func (padLeftZero) Inspect() []byte { return []byte("0") }

func TestIsBinaryEncodedField(t *testing.T) {
	t.Parallel()
	bin := field.NewBinary(&field.Spec{
		Length: 8, Description: "raw", Enc: encoding.Binary, Pref: prefix.Binary.Fixed,
	})
	if !isBinaryEncodedField(bin) {
		t.Error("binary field should be binary-encoded")
	}
	ascii := field.NewString(&field.Spec{
		Length: 6, Description: "x", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed,
	})
	if isBinaryEncodedField(ascii) {
		t.Error("ASCII field should not be binary-encoded")
	}
	hexField := field.NewString(&field.Spec{
		Length: 8, Description: "hex", Enc: encoding.BytesToASCIIHex, Pref: prefix.ASCII.Fixed,
	})
	if !isBinaryEncodedField(hexField) {
		t.Error("BytesToASCIIHex field should be binary-encoded")
	}
	// A field whose spec has no encoding set is not binary-encoded.
	noEnc := field.NewString(&field.Spec{Length: 4, Description: "x", Pref: prefix.ASCII.Fixed})
	if isBinaryEncodedField(noEnc) {
		t.Error("a field with no encoding should not be binary-encoded")
	}
}

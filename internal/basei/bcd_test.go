package basei

import (
	"strings"
	"testing"

	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
)

// TestSpec87BCDStarterShape pins the packed-BCD starter layout so the preset
// stays usable for raw-binary captures: field 55 is an EMV TLV composite, the
// PIN and MAC fields carry raw bytes, and variable-length fields encode their
// length prefix as BCD rather than ASCII.
func TestSpec87BCDStarterShape(t *testing.T) {
	t.Parallel()
	spec := Spec87BCDStarter()

	// field 55 must be a TLV composite so 55.<tag> packs.
	if _, ok := spec.Fields[55].(*field.Composite); !ok {
		t.Errorf("field 55 is %T, want *field.Composite", spec.Fields[55])
	}

	// PIN and MAC must be raw fixed-length binary.
	for _, id := range []int{52, 64} {
		s := spec.Fields[id].Spec()
		if s.Enc != encoding.Binary {
			t.Errorf("field %d encoding is %T, want Binary", id, s.Enc)
		}
		if got := s.Pref.Inspect(); got != "Binary.Fixed" {
			t.Errorf("field %d prefix is %q, want Binary.Fixed", id, got)
		}
	}

	// variable-length fields must not keep an ASCII length prefix.
	for _, id := range []int{32, 35, 36, 45} {
		got := spec.Fields[id].Spec().Pref.Inspect()
		if strings.HasPrefix(got, "ASCII.") {
			t.Errorf("field %d still uses an ASCII length prefix %q in the packed-BCD preset", id, got)
		}
		if !strings.HasPrefix(got, "BCD.") {
			t.Errorf("field %d prefix is %q, want a BCD length prefix", id, got)
		}
	}
}

// TestSpec87BCDStarterNumericFieldsArePackedBCD pins that the numeric
// secondary-bitmap fields encode their payload as packed BCD, not ASCII: a
// digit string "1234" becomes the two bytes 0x12 0x34 (hex "1234"), never the
// four ASCII bytes 0x31 0x32 0x33 0x34 (hex "31323334").
func TestSpec87BCDStarterNumericFieldsArePackedBCD(t *testing.T) {
	t.Parallel()
	spec := Spec87BCDStarter()

	cases := []struct {
		id    int
		value string
		want  string // expected packed-BCD payload, hex (no length prefix for fixed)
	}{
		{71, "1234", "1234"},
		{74, "0000000001", "0000000001"},
		{82, "000000000001", "000000000001"},
		{86, "0000000000000001", "0000000000000001"},
		{97, "00000000000000001", "000000000000000001"}, // 17 digits -> 9 bytes, left-padded
		{90, "020022334406041301050000000000000000000000", "020022334406041301050000000000000000000000"},
		{95, "000000000000000000000000000000000000000000", "000000000000000000000000000000000000000000"},
	}
	for _, tc := range cases {
		f := spec.Fields[tc.id]
		if err := f.SetBytes([]byte(tc.value)); err != nil {
			t.Fatalf("field %d SetBytes: %v", tc.id, err)
		}
		packed, err := f.Pack()
		if err != nil {
			t.Fatalf("field %d Pack: %v", tc.id, err)
		}
		got := strings.ToLower(hexString(packed))
		if got != strings.ToLower(tc.want) {
			t.Errorf("field %d packed = %s, want %s (packed BCD, not ASCII)", tc.id, got, tc.want)
		}
		// An ASCII encoding of "1234" would be "31323334"; make sure we are not that.
		if strings.Contains(got, "313233") {
			t.Errorf("field %d looks ASCII-encoded: %s", tc.id, got)
		}
	}
}

func hexString(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = digits[c>>4]
		out[i*2+1] = digits[c&0xf]
	}
	return string(out)
}

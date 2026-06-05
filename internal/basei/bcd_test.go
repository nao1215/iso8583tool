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

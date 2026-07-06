package basei

import (
	"testing"

	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"
)

// TestRawBinaryFieldDescriptionFallback exercises the src-derived description
// branch: when description is empty and src carries a spec, its description is
// reused.
func TestRawBinaryFieldDescriptionFallback(t *testing.T) {
	t.Parallel()

	src := field.NewString(&field.Spec{
		Length:      16,
		Description: "PIN Data",
		Enc:         encoding.ASCII,
		Pref:        prefix.ASCII.Fixed,
	})
	got := rawBinaryField(src, 8, "")
	if got.Spec().Description != "PIN Data" {
		t.Errorf("description = %q, want %q", got.Spec().Description, "PIN Data")
	}
	if got.Spec().Length != 8 {
		t.Errorf("length = %d, want 8", got.Spec().Length)
	}
	if _, ok := got.(*field.Binary); !ok {
		t.Errorf("rawBinaryField returned %T, want *field.Binary", got)
	}

	// Explicit description wins over the src spec's description, and a different
	// byte length is honored.
	named := rawBinaryField(src, 16, "MAC")
	if named.Spec().Description != "MAC" {
		t.Errorf("description = %q, want %q", named.Spec().Description, "MAC")
	}
	if named.Spec().Length != 16 {
		t.Errorf("length = %d, want 16", named.Spec().Length)
	}
}

// TestBCDLengthPrefix covers every ASCII length prefix mapping plus the
// nil-returning default for a fixed/binary prefix.
func TestBCDLengthPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   prefix.Prefixer
		want prefix.Prefixer
	}{
		{prefix.ASCII.L, prefix.BCD.L},
		{prefix.ASCII.LL, prefix.BCD.LL},
		{prefix.ASCII.LLL, prefix.BCD.LLL},
		{prefix.ASCII.LLLL, prefix.BCD.LLLL},
	}
	for _, c := range cases {
		if got := bcdLengthPrefix(c.in); got != c.want {
			t.Errorf("bcdLengthPrefix(%s) = %v, want %v", c.in.Inspect(), got, c.want)
		}
	}
	if got := bcdLengthPrefix(prefix.ASCII.Fixed); got != nil {
		t.Errorf("bcdLengthPrefix(fixed) = %v, want nil", got)
	}
	if got := bcdLengthPrefix(prefix.Binary.Fixed); got != nil {
		t.Errorf("bcdLengthPrefix(binary fixed) = %v, want nil", got)
	}
}

// TestCloneWithEncoding covers the Numeric, String, and default (unchanged)
// branches of cloneWithEncoding.
func TestCloneWithEncoding(t *testing.T) {
	t.Parallel()

	numeric := field.NewNumeric(&field.Spec{
		Length:      6,
		Description: "amount",
		Enc:         encoding.ASCII,
		Pref:        prefix.ASCII.Fixed,
	})
	clonedNum := cloneWithEncoding(numeric, encoding.BCD, prefix.BCD.Fixed)
	if _, ok := clonedNum.(*field.Numeric); !ok {
		t.Errorf("cloned numeric = %T, want *field.Numeric", clonedNum)
	}
	if clonedNum.Spec().Enc != encoding.BCD {
		t.Errorf("cloned numeric enc not swapped to BCD")
	}

	str := field.NewString(&field.Spec{
		Length:      10,
		Description: "text",
		Enc:         encoding.ASCII,
		Pref:        prefix.ASCII.LL,
	})
	clonedStr := cloneWithEncoding(str, encoding.BCD, prefix.BCD.LL)
	if _, ok := clonedStr.(*field.String); !ok {
		t.Errorf("cloned string = %T, want *field.String", clonedStr)
	}

	// A non-String/Numeric field (Binary) is returned unchanged.
	bin := field.NewBinary(&field.Spec{
		Length:      8,
		Description: "raw",
		Enc:         encoding.Binary,
		Pref:        prefix.Binary.Fixed,
	})
	clonedBin := cloneWithEncoding(bin, encoding.Binary, prefix.Binary.Fixed)
	if clonedBin != bin {
		t.Errorf("cloneWithEncoding on Binary should return the same field unchanged")
	}
}

package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/messageio"
)

// TestMaskEmbeddedSensitive covers the embedded-PAN scanner: Luhn-valid and
// key-labeled PANs (contiguous or separated) are masked, while plain numeric
// identifiers are left intact.
func TestMaskEmbeddedSensitive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		masked  bool
		keepRaw string // a substring that must NOT survive when masked
	}{
		{"luhn contiguous", "PAN=4111111111111111", true, "4111111111111111"},
		{"luhn dashed", "PAN=4111-1111-1111-1111", true, "1111-1111"},
		{"luhn spaced", "PAN=4111 1111 1111 1111", true, "1111 1111 1111"},
		{"labeled non-luhn", "PAN=4222222222222222", true, "4222222222222222"},
		{"business id", "ORDER_ID=1234567890123|TOKEN=ABC", false, ""},
		{"short number", "QTY=12345", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := maskEmbeddedSensitive(tc.in)
			if tc.masked {
				if got == tc.in {
					t.Fatalf("expected %q to be masked, got unchanged", tc.in)
				}
				if tc.keepRaw != "" && strings.Contains(got, tc.keepRaw) {
					t.Fatalf("masked output %q still contains raw %q", got, tc.keepRaw)
				}
			} else if got != tc.in {
				t.Fatalf("expected %q unchanged, got %q", tc.in, got)
			}
		})
	}
}

// TestMaskCardholderFieldList pins the positional field list: PAN fields 2 and
// 34 are masked, the track fields are masked, and field 20 (a country code, not
// a PAN) is left intact.
func TestMaskCardholderFieldList(t *testing.T) {
	t.Parallel()
	doc := messageio.Document{
		MTI: "0100",
		Fields: map[string]string{
			"2":  "4111111111111111",
			"34": "411111111111111111111111",
			"20": "840",
			"35": "4111111111111111D29122011234567890",
		},
	}
	MaskCardholderData(&doc)
	if doc.Fields["20"] != "840" {
		t.Errorf("field 20 (country code) must not be masked, got %q", doc.Fields["20"])
	}
	for _, id := range []string{"2", "34", "35"} {
		if strings.ContainsAny(strings.TrimLeft(doc.Fields[id], "*"), "0123456789") && !strings.Contains(doc.Fields[id], "*") {
			t.Errorf("field %s should be masked, got %q", id, doc.Fields[id])
		}
		if !strings.Contains(doc.Fields[id], "*") {
			t.Errorf("field %s should contain mask characters, got %q", id, doc.Fields[id])
		}
	}
}

// TestMaskBinaryRepresentation checks that a PAN/track carried as a binary
// (hex-encoded) field value is masked, not only the text representation.
func TestMaskBinaryRepresentation(t *testing.T) {
	t.Parallel()
	doc := messageio.Document{
		MTI: "0100",
		BinaryFields: map[string]string{
			"2":  "34313131313131313131313131313131",         // "4111111111111111"
			"63": "50414E3D34313131313131313131313131313131", // "PAN=4111111111111111"
		},
	}
	MaskCardholderData(&doc)
	if strings.Trim(doc.BinaryFields["2"], "*") != "" {
		t.Errorf("binary PAN field 2 should be fully masked, got %q", doc.BinaryFields["2"])
	}
	if strings.Trim(doc.BinaryFields["63"], "*") != "" {
		t.Errorf("binary private field 63 embedding a PAN should be masked, got %q", doc.BinaryFields["63"])
	}
}

// TestMaskSensitiveTLVTagAnyContainer checks that a sensitive TLV tag is masked
// wherever it nests: directly under 55, under a different container (127), and
// inside a constructed template (55.70).
func TestMaskSensitiveTLVTagAnyContainer(t *testing.T) {
	t.Parallel()
	track := "34313131313131313131313131313131" // hex bytes
	doc := messageio.Document{
		MTI: "0110",
		BinaryFields: map[string]string{
			"55.9F6B":  track,
			"127.57":   track,
			"55.70.57": track,
		},
	}
	MaskCardholderData(&doc)
	for _, path := range []string{"55.9F6B", "127.57", "55.70.57"} {
		if strings.Trim(doc.BinaryFields[path], "*") != "" {
			t.Errorf("sensitive tag %s should be fully masked, got %q", path, doc.BinaryFields[path])
		}
	}
}

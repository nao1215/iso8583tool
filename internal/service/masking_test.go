package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/render"
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
		// Free-form track labels must mask the whole track, not just the PAN, so the
		// expiry/service/discretionary tail does not leak.
		{"track2 label", "TRACK2=4111111111111111D29122011234567890", true, "29122011234567890"},
		{"track1 label", "TRACK1=B4111111111111111^DOE/J^29122011", true, "29122011"},
		{"track no number", "TRACK=4111111111111111D2912", true, "2912"},
		// PAN key-label variants commonly seen in real logs.
		{"underscore card_no", "card_no=4222222222222222", true, "4222222222222222"},
		{"underscore pan_no", "pan_no=4222222222222222", true, "4222222222222222"},
		{"underscore account_no", "account_no=4222222222222222", true, "4222222222222222"},
		{"spaced card number", "card number=4222222222222222", true, "4222222222222222"},
		{"spaced acct number", "acct number=4222222222222222", true, "4222222222222222"},
		{"hyphen card-number", "card-number=4222222222222222", true, "4222222222222222"},
		{"long snake pan", "primary_account_number=4222222222222222", true, "4222222222222222"},
		{"camel pan", "primaryAccountNumber=4222222222222222", true, "4222222222222222"},
		// "discard" must not be read as a "card" label (non-Luhn digits stay intact).
		{"discard not a card label", "discard=1234567890123456", false, ""},
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
	MaskCardholderData(&doc, true)
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
	MaskCardholderData(&doc, true)
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
	MaskCardholderData(&doc, true)
	for _, path := range []string{"55.9F6B", "127.57", "55.70.57"} {
		if strings.Trim(doc.BinaryFields[path], "*") != "" {
			t.Errorf("sensitive tag %s should be fully masked, got %q", path, doc.BinaryFields[path])
		}
	}
}

// TestDescribeDoesNotOverMaskSubfields guards the masking regression where the
// full describe view masked a custom composite subfield whose local id collided
// with a top-level cardholder field (48.2 vs PAN field 2): the top-level mask
// must apply to the top-level field only, while a same-numbered subfield keeps
// its real value.
func TestDescribeDoesNotOverMaskSubfields(t *testing.T) {
	t.Parallel()

	spec := loadSpecFile(t, "f48-positional.json", `{
  "name": "F48 positional",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "2": {"type":"String","length":16,"description":"PAN","enc":"ASCII","prefix":"ASCII.LL"},
    "11": {"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},
    "48": {
      "type":"Composite","length":999,"description":"Private Data","prefix":"ASCII.LLL",
      "tag":{"sort":"StringsByInt"},
      "subfields": {
        "1": {"type":"String","length":3,"description":"A","enc":"ASCII","prefix":"ASCII.Fixed"},
        "2": {"type":"String","length":2,"description":"B","enc":"ASCII","prefix":"ASCII.Fixed"}
      }
    }
  }
}`)

	doc := messageio.Document{
		MTI:    "0100",
		Fields: map[string]string{"2": "4111111111111111", "11": "123456", "48.1": "ABC", "48.2": "DE"},
	}
	packed, err := WriteMessage(doc, spec)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	res, err := ViewMessage(packed.Raw, spec, basei.ExtensionCatalog{}, "describe", nil, render.NewPalette(false), false)
	if err != nil {
		t.Fatalf("ViewMessage: %v", err)
	}
	// Top-level PAN field 2 is masked...
	if strings.Contains(res.Body, "4111111111111111") {
		t.Fatalf("top-level PAN must be masked:\n%s", res.Body)
	}
	// ...but the composite subfield 48.2 ("DE") must NOT be masked.
	if !strings.Contains(res.Body, "48.2   B: DE") && !strings.Contains(res.Body, "48.2  B: DE") && !strings.Contains(res.Body, "DE") {
		t.Fatalf("subfield 48.2 should keep its real value DE:\n%s", res.Body)
	}
	if strings.Contains(res.Body, "48.2   B: **") {
		t.Fatalf("subfield 48.2 must not be masked by the top-level PAN rule:\n%s", res.Body)
	}
}

// TestMaskCustomSpecSemantics covers the custom-spec masking model: the BASE I
// field-id and EMV-tag rules do not apply (so a harmless custom field 35/52 is
// not masked), but content scanning still masks anything PAN- or track-shaped,
// so a real PAN in any field never leaks.
func TestMaskCustomSpecSemantics(t *testing.T) {
	t.Parallel()

	t.Run("harmless custom fields are left intact", func(t *testing.T) {
		t.Parallel()
		doc := messageio.Document{
			MTI: "0110",
			Fields: map[string]string{
				"35": "REF-ORDER-ABC-0001", // not track data under this custom spec
				"52": "ABCDEFGH",           // not a PIN under this custom spec
			},
		}
		MaskCardholderData(&doc, false)
		if doc.Fields["35"] != "REF-ORDER-ABC-0001" {
			t.Errorf("custom field 35 must not be masked, got %q", doc.Fields["35"])
		}
		if doc.Fields["52"] != "ABCDEFGH" {
			t.Errorf("custom field 52 must not be masked, got %q", doc.Fields["52"])
		}
	})

	t.Run("a real PAN is masked in any field", func(t *testing.T) {
		t.Parallel()
		doc := messageio.Document{
			MTI: "0110",
			Fields: map[string]string{
				"2":  "4111111111111111",     // a Luhn-valid PAN
				"35": "PAN=4111111111111111", // a labeled PAN
			},
		}
		MaskCardholderData(&doc, false)
		if strings.Contains(doc.Fields["2"], "4111111111111111") {
			t.Errorf("a real PAN must be masked even under a custom spec, got %q", doc.Fields["2"])
		}
		if strings.Contains(doc.Fields["35"], "4111111111111111") {
			t.Errorf("a labeled PAN must be masked even under a custom spec, got %q", doc.Fields["35"])
		}
	})

	t.Run("built-in semantics still mask field 35 and 52", func(t *testing.T) {
		t.Parallel()
		doc := messageio.Document{
			MTI:    "0110",
			Fields: map[string]string{"35": "4111111111111111D2912", "52": "1234ABCD"},
		}
		MaskCardholderData(&doc, true)
		if doc.Fields["35"] == "4111111111111111D2912" {
			t.Errorf("built-in field 35 (track) must be masked")
		}
		if doc.Fields["52"] == "1234ABCD" {
			t.Errorf("built-in field 52 (PIN) must be masked")
		}
	})
}

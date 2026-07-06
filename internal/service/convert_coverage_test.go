package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
)

// TestMessageToDocumentBinaryPrimitives round-trips a packed-BCD message that
// carries raw binary primitive fields (PIN 52, MAC 64), exercising the
// isBinaryEncodedField branch of appendFieldToDocument that emits them as hex
// under binary_fields.
func TestMessageToDocumentBinaryPrimitives(t *testing.T) {
	t.Parallel()

	spec := basei.Spec87BCDStarter()
	doc := messageio.Document{
		MTI: "0100",
		Fields: map[string]string{
			"3":  "000000",
			"4":  "000000005000",
			"11": "123456",
			"41": "TERMID01",
		},
		BinaryFields: map[string]string{
			"52": "1122334455667788",
			"64": "AABBCCDDEEFF0011",
		},
	}

	packed, err := WriteMessage(doc, spec)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	back, err := MessageToDocument(spec, packed.Raw)
	if err != nil {
		t.Fatalf("MessageToDocument: %v", err)
	}
	if back.BinaryFields["52"] != "1122334455667788" {
		t.Errorf("F52 = %q, want raw PIN hex", back.BinaryFields["52"])
	}
	if back.BinaryFields["64"] != "AABBCCDDEEFF0011" {
		t.Errorf("F64 = %q, want raw MAC hex", back.BinaryFields["64"])
	}
	if back.Fields["4"] != "000000005000" {
		t.Errorf("F4 = %q, want canonical amount", back.Fields["4"])
	}
}

// TestMessageToDocumentUnpackError covers the error branch where the raw bytes
// cannot be unpacked under the spec, so a field-aware diagnosis is returned.
func TestMessageToDocumentUnpackError(t *testing.T) {
	t.Parallel()

	if _, err := MessageToDocument(basei.StarterMessageSpec(), []byte{0x01, 0x02}); err == nil {
		t.Fatal("MessageToDocument of truncated bytes should error")
	}
}

// TestMessageToDocumentUnknownTags round-trips a message with an unknown BER-TLV
// tag under field 55 so the UnknownTags loop keeps it in binary_fields.
func TestMessageToDocumentUnknownTags(t *testing.T) {
	t.Parallel()

	spec := basei.StarterMessageSpec()
	doc := basei.AuthRequest()
	doc.BinaryFields["55.DF8130"] = "DEADBEEF"

	packed, err := WriteMessage(doc, spec)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	back, err := MessageToDocument(spec, packed.Raw)
	if err != nil {
		t.Fatalf("MessageToDocument: %v", err)
	}
	got, ok := back.BinaryFields["55.DF8130"]
	if !ok || !strings.EqualFold(got, "DEADBEEF") {
		t.Errorf("unknown tag 55.DF8130 = %q,%v; want DEADBEEF preserved", got, ok)
	}
}

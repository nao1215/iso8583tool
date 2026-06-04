package service

import (
	"testing"

	"github.com/moov-io/iso8583"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/messagespec"
	"github.com/nao1215/iso8583tool/internal/render"
)

// unknownTLVDocument mirrors examples/basei/0100-auth-request-unknown-tlv.json.
// Field 55 is supplied per tag, including DF8129, which is intentionally absent
// from the basei-starter known-tag set, so unknown-tag editing can be tested.
func unknownTLVDocument() messageio.Document {
	return messageio.Document{
		MTI: "0100",
		Fields: map[string]string{
			"2":  "4111111111111111",
			"11": "123456",
			"41": "TERMID01",
		},
		BinaryFields: map[string]string{
			"55.82":     "3900",
			"55.95":     "8000008000",
			"55.9F02":   "000000005000",
			"55.9F36":   "0034",
			"55.DF8129": "AABBCCDD",
		},
	}
}

func TestUnknownTLVRoundTrip(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load returned error: %v", err)
	}

	writeResult, err := WriteMessage(unknownTLVDocument(), spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	// validate must surface the unknown tag as a warning, not an error.
	report := ValidateMessage(writeResult.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), false)
	if report.HasErrors() {
		t.Fatalf("ValidateMessage returned errors: %#v", report.Issues)
	}
	if len(report.UnknownTags) != 1 || report.UnknownTags[0].Path != "55.DF8129" {
		t.Fatalf("expected unknown tag 55.DF8129, got %#v", report.UnknownTags)
	}
	// The unknown tag's bytes can hold cardholder data, so the report masks
	// them (length-preserving) while still flagging the tag path.
	if report.UnknownTags[0].Raw != "********" {
		t.Fatalf("unknown tag raw = %q, want it masked to ********", report.UnknownTags[0].Raw)
	}
	if !hasUnknownTagWarning(report) {
		t.Fatalf("expected a warning issue for the unknown tag, got %#v", report.Issues)
	}

	// view must list the preserved unknown tag too.
	viewResult, err := ViewMessage(writeResult.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false))
	if err != nil {
		t.Fatalf("ViewMessage returned error: %v", err)
	}
	if len(viewResult.UnknownTags) != 1 || viewResult.UnknownTags[0].Path != "55.DF8129" {
		t.Fatalf("expected view to report 55.DF8129, got %#v", viewResult.UnknownTags)
	}

	// round-trip: unpack then re-pack must preserve every byte, unknown tag included.
	msg := iso8583.NewMessage(spec.MessageSpec)
	if err := msg.Unpack(writeResult.Raw); err != nil {
		t.Fatalf("Unpack returned error: %v", err)
	}
	repacked, err := msg.Pack()
	if err != nil {
		t.Fatalf("Pack returned error: %v", err)
	}
	if string(repacked) != string(writeResult.Raw) {
		t.Fatal("round-trip changed the packed bytes; unknown TLV tag was not preserved")
	}
}

func hasUnknownTagWarning(report ValidationReport) bool {
	for _, issue := range report.Issues {
		if issue.Severity == "warning" && issue.Path == "55.DF8129" {
			return true
		}
	}
	return false
}

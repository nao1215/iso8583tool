package service

import (
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/messagespec"
)

// strictReport packs a document with the default spec and validates it strictly.
func strictReport(t *testing.T, doc messageio.Document) ValidationReport {
	t.Helper()
	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	packed, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage(%s): %v", doc.MTI, err)
	}
	return ValidateMessage(packed.Raw, spec.MessageSpec, "basei-starter", basei.DefaultExtensionCatalog(), true)
}

// TestStrictRejectsHollowAdviceAndNetwork covers the advice and network-management
// MTIs that previously slipped through --strict with only a STAN.
func TestStrictRejectsHollowAdviceAndNetwork(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mti    string
		fields map[string]string
	}{
		{"0120", map[string]string{"11": "123456"}},             // authorization advice
		{"0220", map[string]string{"11": "123456"}},             // financial advice
		{"0820", map[string]string{"11": "123456"}},             // network advice
		{"0810", map[string]string{"11": "123456", "39": "00"}}, // network response
		{"0830", map[string]string{"11": "123456", "39": "00"}}, // network advice response
	}
	for _, tc := range cases {
		t.Run(tc.mti, func(t *testing.T) {
			t.Parallel()
			report := strictReport(t, messageio.Document{MTI: tc.mti, Fields: tc.fields})
			if !report.HasErrors() {
				t.Fatalf("hollow %s should fail --strict, got %#v", tc.mti, report.Issues)
			}
		})
	}
}

// TestStrictAcceptsCompleteAdviceAndNetwork guards against false positives: a
// well-formed advice or network-management message must still validate.
func TestStrictAcceptsCompleteAdviceAndNetwork(t *testing.T) {
	t.Parallel()

	cases := []messageio.Document{
		// authorization / financial advice with a PAN source and the request core.
		{MTI: "0120", Fields: map[string]string{"2": "4111111111111111", "3": "000000", "4": "000000001000", "7": "0605123456", "11": "123456"}},
		{MTI: "0220", Fields: map[string]string{"2": "4111111111111111", "3": "000000", "4": "000000001000", "7": "0605123456", "11": "123456"}},
		// network-management messages carrying field 70.
		{MTI: "0820", Fields: map[string]string{"11": "123456", "70": "301"}},
		{MTI: "0810", Fields: map[string]string{"11": "123456", "39": "00", "70": "301"}},
		{MTI: "0830", Fields: map[string]string{"11": "123456", "39": "00", "70": "301"}},
	}
	for _, doc := range cases {
		t.Run(doc.MTI, func(t *testing.T) {
			t.Parallel()
			report := strictReport(t, doc)
			if report.HasErrors() {
				t.Fatalf("complete %s should pass --strict, got %#v", doc.MTI, report.Issues)
			}
		})
	}
}

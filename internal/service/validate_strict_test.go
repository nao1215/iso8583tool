package service

import (
	"strings"
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

// TestStrictRejectsHollowNotificationInstructionAndFileAction covers the MTI
// functions (notification, ack, instruction, ack) and classes (file action,
// reversal) that previously passed --strict with only a STAN.
func TestStrictRejectsHollowNotificationInstructionAndFileAction(t *testing.T) {
	t.Parallel()

	// Every MTI here carries only field 11, so strict must report an error.
	for _, mti := range []string{
		"0140", "0150", "0160", "0170", // authorization notification/ack/instruction/ack
		"0240", "0250", "0260", "0270", // financial notification/ack/instruction/ack
		"0300", "0320", "0340", "0360", // file action requests/notifications/instructions
		"0310", "0330", "0350", "0370", // file action responses/acks
		"0440", "0450", "0460", "0470", // reversal notification/ack/instruction/ack
	} {
		report := strictReport(t, messageio.Document{MTI: mti, Fields: map[string]string{"11": "123456"}})
		if report.Valid {
			t.Errorf("a hollow %s must fail strict validation, got valid", mti)
		}
	}
}

// TestStrictReversalRequiresPANSource pins that a reversal request/advice must
// carry a PAN source (field 2, 35, or 45), tying it to the original transaction.
func TestStrictReversalRequiresPANSource(t *testing.T) {
	t.Parallel()

	for _, mti := range []string{"0400", "0420"} {
		doc := messageio.Document{MTI: mti, Fields: map[string]string{
			"4": "000000001000", "7": "0605123456", "11": "123456",
			"90": "020022334406041301050000000000000000000000",
		}}
		report := strictReport(t, doc)
		if report.Valid {
			t.Errorf("a PAN-less reversal %s must fail strict validation", mti)
		}
		// Adding a PAN makes it pass.
		doc.Fields["2"] = "4111111111111111"
		if report := strictReport(t, doc); !report.Valid {
			t.Errorf("a reversal %s with a PAN should pass strict, issues=%v", mti, report.Issues)
		}
	}
}

// TestStrictWarnsOnUnmodeledClasses pins that reconciliation/administrative/fee
// collection messages are flagged (warning) rather than silently passing as
// fully validated.
func TestStrictWarnsOnUnmodeledClasses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ mti, want string }{
		{"0500", "reconciliation"},
		{"0600", "administrative"},
		{"0700", "fee collection"},
	} {
		report := strictReport(t, messageio.Document{MTI: tc.mti, Fields: map[string]string{"11": "123456"}})
		found := false
		for _, issue := range report.Issues {
			if issue.Severity == SeverityWarning && strings.Contains(issue.Message, tc.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s should warn that strict rules for %s are not implemented, issues=%v", tc.mti, tc.want, report.Issues)
		}
	}
}

// TestStrictRejectsAlphabeticNumericFields pins that a numeric field carrying an
// alphabetic value fails strict validation. moov models these fields as String,
// so without the digit check an alphabetic value packs and passes.
func TestStrictRejectsAlphabeticNumericFields(t *testing.T) {
	t.Parallel()

	base := map[string]string{
		"2": "4111111111111111", "3": "000000", "4": "000000001000",
		"7": "0605123456", "11": "123456",
	}
	cases := []struct{ id, value string }{
		{"70", "ABC"},
		{"90", "NOTNUMERICNOTNUMERICNOTNUMERICNOTNUMERIC12"},
		{"95", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{"100", "HELLOWORLD1"},
	}
	for _, tc := range cases {
		fields := map[string]string{}
		for k, v := range base {
			fields[k] = v
		}
		fields[tc.id] = tc.value
		report := strictReport(t, messageio.Document{MTI: "0200", Fields: fields})
		if report.Valid {
			t.Errorf("an alphabetic field %s must fail strict validation", tc.id)
		}
		found := false
		for _, issue := range report.Issues {
			if issue.Path == tc.id && strings.Contains(issue.Message, "must be numeric") {
				found = true
			}
		}
		if !found {
			t.Errorf("field %s should report a numeric-only error, issues=%v", tc.id, report.Issues)
		}
	}

	// A digit value still passes.
	ok := strictReport(t, messageio.Document{MTI: "0800", Fields: map[string]string{"11": "123456", "70": "301"}})
	for _, issue := range ok.Issues {
		if strings.Contains(issue.Message, "must be numeric") {
			t.Errorf("a digit value must not trigger the numeric check: %v", issue)
		}
	}
}

package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/render"
)

func indexDiff(changes []DiffEntry) map[string]DiffEntry {
	got := make(map[string]DiffEntry, len(changes))
	for _, c := range changes {
		got[c.Path] = c
	}
	return got
}

// TestDiffMasksSensitiveValues locks in that diff is as safe to share as view
// and redact: by default it detects changes on the real values but never prints
// a full PAN, full track, or unknown TLV bytes. --unsafe opts back into raw.
func TestDiffMasksSensitiveValues(t *testing.T) {
	t.Parallel()
	spec := diffSpec(t)

	before := basei.AuthRequest()
	after := basei.AuthRequest()
	after.Fields["2"] = "4222222222222222"
	after.Fields["35"] = "4222222222222222D29122011234567890"
	before.BinaryFields["55.9F6B"] = "1111111111111111D2512201"
	after.BinaryFields["55.9F6B"] = "2222222222222222D2512201"

	beforeRaw, err := WriteMessage(before, spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack before: %v", err)
	}
	afterRaw, err := WriteMessage(after, spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack after: %v", err)
	}

	safe, err := DiffMessages(spec.MessageSpec, beforeRaw.Raw, afterRaw.Raw, nil, false)
	if err != nil {
		t.Fatalf("DiffMessages: %v", err)
	}
	got := indexDiff(safe.Changes)

	// PAN: change detected, displayed as BIN + last four only.
	if c, ok := got["2"]; !ok || c.Kind != DiffChanged {
		t.Fatalf("F2 should be changed: %#v", c)
	} else if c.Before != "411111******1111" || c.After != "422222******2222" {
		t.Fatalf("F2 not masked to BIN+last4: %q -> %q", c.Before, c.After)
	}

	// Track 2: must keep no full PAN on either side.
	c35 := got["35"]
	if strings.Contains(c35.Before, "4111111111111111") || strings.Contains(c35.After, "4222222222222222") {
		t.Fatalf("F35 leaked the full PAN: %#v", c35)
	}

	// Unknown TLV: change is still detected (raw comparison) but bytes are masked.
	if c, ok := got["55.9F6B"]; !ok || c.Kind != DiffChanged {
		t.Fatalf("55.9F6B should be detected as changed: %#v", c)
	} else if strings.Trim(c.Before, "*") != "" || strings.Trim(c.After, "*") != "" {
		t.Fatalf("55.9F6B should be fully masked: %#v", c)
	}

	// --unsafe shows the real values for local debugging.
	unsafe, err := DiffMessages(spec.MessageSpec, beforeRaw.Raw, afterRaw.Raw, nil, true)
	if err != nil {
		t.Fatalf("DiffMessages unsafe: %v", err)
	}
	gotU := indexDiff(unsafe.Changes)
	if gotU["2"].After != "4222222222222222" {
		t.Fatalf("--unsafe F2 should show the full PAN, got %q", gotU["2"].After)
	}
	if !strings.Contains(gotU["55.9F6B"].After, "2222222222222222") {
		t.Fatalf("--unsafe 55.9F6B should show raw bytes, got %q", gotU["55.9F6B"].After)
	}
}

// TestValidateStrictFlagsHollowResponse verifies the difference between lenient
// (does it unpack?) and strict (is it a well-formed BASE I message?) modes.
func TestValidateStrictFlagsHollowResponse(t *testing.T) {
	t.Parallel()
	spec := diffSpec(t)

	doc := messageio.Document{MTI: "0110", Fields: map[string]string{"11": "123456"}}
	raw, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	lenient := ValidateMessage(raw.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), false)
	if !lenient.Valid {
		t.Fatalf("lenient validation should pass a message that unpacks, issues=%#v", lenient.Issues)
	}

	strict := ValidateMessage(raw.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), true)
	if strict.Valid {
		t.Fatalf("strict validation should fail a 0110 carrying only a STAN")
	}
	if !hasIssue(strict, "error", "39") {
		t.Fatalf("strict should flag the missing response code (field 39): %#v", strict.Issues)
	}
}

// TestValidateStrictAcceptsBundledSamples ensures the strict rules do not raise
// false positives on the tool's own well-formed BASE I fixtures.
func TestValidateStrictAcceptsBundledSamples(t *testing.T) {
	t.Parallel()
	spec := diffSpec(t)

	samples := []messageio.Document{
		basei.AuthRequest(), basei.AuthResponse(),
		basei.FinancialRequest(), basei.FinancialResponse(),
		basei.ReversalAdvice(), basei.ReversalResponse(),
		basei.NetworkEchoRequest(), basei.NetworkEchoResponse(),
	}
	for _, doc := range samples {
		raw, err := WriteMessage(doc, spec.MessageSpec)
		if err != nil {
			t.Fatalf("pack %s: %v", doc.MTI, err)
		}
		report := ValidateMessage(raw.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), true)
		if report.HasErrors() {
			t.Fatalf("strict validation should accept sample %s, issues=%#v", doc.MTI, report.Issues)
		}
	}
}

// TestViewMasksOnlyUnknownTagValues guards against the over-broad masking that a
// body-wide string replace would cause: a known tag must keep its bytes even
// when an unknown tag happens to share the same value.
func TestViewMasksOnlyUnknownTagValues(t *testing.T) {
	t.Parallel()
	spec := diffSpec(t)

	doc := messageio.Document{
		MTI:    "0110",
		Fields: map[string]string{"11": "123456", "39": "00"},
		BinaryFields: map[string]string{
			"55.8A":   "3030", // known tag (Authorisation Response Code)
			"55.DF01": "3030", // unknown tag with identical bytes
		},
	}
	raw, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	res, err := ViewMessage(raw.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false), false)
	if err != nil {
		t.Fatalf("ViewMessage: %v", err)
	}

	if !lineWith(res.Body, "Authorisation Response Code", "3030") {
		t.Fatalf("known tag 8A must keep its value:\n%s", res.Body)
	}
	if lineWith(res.Body, "Unknown TLV tag DF01", "3030") {
		t.Fatalf("unknown tag DF01 leaked its bytes:\n%s", res.Body)
	}
	if !lineWith(res.Body, "Unknown TLV tag DF01", "****") {
		t.Fatalf("unknown tag DF01 should be masked:\n%s", res.Body)
	}
}

func hasIssue(report ValidationReport, severity, path string) bool {
	for _, issue := range report.Issues {
		if issue.Severity == severity && issue.Path == path {
			return true
		}
	}
	return false
}

// lineWith reports whether some line containing marker also contains value.
func lineWith(body, marker, value string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, marker) && strings.Contains(line, value) {
			return true
		}
	}
	return false
}

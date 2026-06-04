package service

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/messagespec"
	"github.com/nao1215/iso8583tool/internal/render"
)

func TestWriteValidateAndView(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load returned error: %v", err)
	}

	doc := basei.AuthRequest()
	writeResult, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	report := ValidateMessage(writeResult.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), false)
	if report.HasErrors() {
		t.Fatalf("ValidateMessage returned errors: %#v", report.Issues)
	}
	if report.MTI != "0100" {
		t.Fatalf("report.MTI = %q, want 0100", report.MTI)
	}
	if len(report.Extensions) < 3 {
		t.Fatal("expected extension notices for fields 48, 55, and 62")
	}

	viewResult, err := ViewMessage(writeResult.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false), false)
	if err != nil {
		t.Fatalf("ViewMessage returned error: %v", err)
	}
	if viewResult.Body == "" {
		t.Fatal("expected describe output")
	}
	if len(viewResult.UnknownTags) != 0 {
		t.Fatalf("expected no unknown tags, got %#v", viewResult.UnknownTags)
	}
}

func TestWriteValidateAndViewPackedBCDStarter(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Config{Spec: basei.Spec87BCDStarterPreset})
	if err != nil {
		t.Fatalf("messagespec.Load returned error: %v", err)
	}

	doc := messageio.Document{
		MTI: "0100",
		Fields: map[string]string{
			"2":  "4019249999999999",
			"3":  "327327",
			"4":  "000000001138",
			"14": "2204",
		},
	}
	writeResult, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	if got := strings.ToUpper(hex.EncodeToString(writeResult.Raw)); got != "010070040000000000001040192499999999993273270000000011382204" {
		t.Fatalf("packed raw = %s", got)
	}

	report := ValidateMessage(writeResult.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), false)
	if report.HasErrors() {
		t.Fatalf("ValidateMessage returned errors: %#v", report.Issues)
	}

	viewResult, err := ViewMessage(writeResult.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "json", nil, render.NewPalette(false), false)
	if err != nil {
		t.Fatalf("ViewMessage returned error: %v", err)
	}
	if !strings.Contains(viewResult.Body, `"mti": "0100"`) || !strings.Contains(viewResult.Body, `"2": "401924******9999"`) {
		t.Fatalf("unexpected packed-bcd view:\n%s", viewResult.Body)
	}

	gotDoc, err := MessageToDocument(spec.MessageSpec, writeResult.Raw)
	if err != nil {
		t.Fatalf("MessageToDocument returned error: %v", err)
	}
	if gotDoc.MTI != "0100" || gotDoc.Fields["4"] != "000000001138" {
		t.Fatalf("unexpected round-trip doc: %#v", gotDoc)
	}
}

// TestValidateOpaqueIsNotAnIssue guards the rule that an extension field's
// strategy (here F63 opaque) is reported only under "Extension Field Strategy"
// and never raised as a warning Issue; warnings are reserved for real anomalies
// such as unknown TLV tags.
func TestValidateOpaqueIsNotAnIssue(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load returned error: %v", err)
	}

	// AuthResponse carries an opaque F63 but no unknown TLV tags.
	packed, err := WriteMessage(basei.AuthResponse(), spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	report := ValidateMessage(packed.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), false)
	if report.HasErrors() {
		t.Fatalf("unexpected errors: %#v", report.Issues)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("expected no issues for an opaque-only message, got %#v", report.Issues)
	}

	var sawOpaque bool
	for _, ext := range report.Extensions {
		if ext.Field == 63 && ext.Strategy == string(basei.StrategyOpaque) {
			sawOpaque = true
		}
	}
	if !sawOpaque {
		t.Fatalf("expected F63 opaque under Extension Field Strategy, got %#v", report.Extensions)
	}
}

// TestCraftedInputDoesNotPanic feeds deliberately malformed, truncated, and
// length-spoofed messages through the unpack-driven code paths. They must fail
// cleanly (returning an error or an error-carrying report) and never panic.
func TestCraftedInputDoesNotPanic(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load returned error: %v", err)
	}

	crafted := [][]byte{
		{},
		{0x01},
		{0x30, 0x31, 0x30, 0x30}, // "0100", no bitmap
		// MTI + bitmap claiming F55, then a BER-TLV length that overruns the buffer.
		mustHex(t, "010000000000000008000103DF"),
		// MTI + bitmap claiming F2 (LLVAR) with a length prefix far larger than the data.
		mustHex(t, "0100400000000000000099"),
		// Long run of 0xFF: an oversized, nonsensical bitmap/body.
		mustHex(t, strings.Repeat("FF", 64)),
	}

	for i, raw := range crafted {
		// ValidateMessage must absorb the failure into a report, not panic.
		report := ValidateMessage(raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), false)
		if report.Valid {
			t.Fatalf("crafted input %d unexpectedly validated as good", i)
		}
		// MessageToDocument must return an error, not panic.
		if _, err := MessageToDocument(spec.MessageSpec, raw); err == nil {
			t.Fatalf("crafted input %d unexpectedly converted without error", i)
		}
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return b
}

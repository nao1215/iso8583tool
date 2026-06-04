package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messagespec"
	"github.com/nao1215/iso8583tool/internal/render"
)

func TestRedactMasksSensitiveFields(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	raw, err := WriteMessage(basei.AuthRequest(), spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	red, paths, err := RedactMessage(spec.MessageSpec, raw.Raw)
	if err != nil {
		t.Fatalf("RedactMessage: %v", err)
	}

	// PAN keeps BIN + last four, masks the middle.
	if got := red.Fields["2"]; got != "411111******1111" {
		t.Fatalf("PAN redaction = %q, want 411111******1111", got)
	}
	// Track 2 must not leak the full PAN.
	if strings.Contains(red.Fields["35"], "4111111111111111") {
		t.Fatalf("track 2 still contains the PAN: %q", red.Fields["35"])
	}
	if !strings.HasPrefix(red.Fields["35"], "411111") || !strings.Contains(red.Fields["35"], "*") {
		t.Fatalf("track 2 redaction unexpected: %q", red.Fields["35"])
	}
	// The EMV application cryptogram is fully masked.
	if ac := red.BinaryFields["55.9F26"]; ac == "" || strings.Trim(ac, "*") != "" {
		t.Fatalf("9F26 cryptogram should be fully masked, got %q", ac)
	}
	// A non-sensitive field is untouched.
	if red.Fields["4"] != "000000005000" {
		t.Fatalf("amount must not be redacted, got %q", red.Fields["4"])
	}

	pathSet := map[string]bool{}
	for _, p := range paths {
		pathSet[p] = true
	}
	for _, want := range []string{"2", "35", "55.9F26"} {
		if !pathSet[want] {
			t.Fatalf("expected %s in redacted paths, got %v", want, paths)
		}
	}
}

// TestUnknownTagNeverLeaksSensitiveBytes guards the "safe to share" promise:
// an unknown Field 55 tag (here 9F6B, an unmapped Track 2 Data tag holding a
// PAN) must not surface its bytes through any display or share surface, while
// convert still preserves it for round-trip safety.
func TestUnknownTagNeverLeaksSensitiveBytes(t *testing.T) {
	t.Parallel()

	const pan = "4111111111111111"
	const track2 = pan + "D30122011000000000000F"

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	doc := unknownTLVDocument()
	doc.BinaryFields["55.9F6B"] = track2
	raw, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	// redact (json document) must mask the unknown tag and report its path.
	red, paths, err := RedactMessage(spec.MessageSpec, raw.Raw)
	if err != nil {
		t.Fatalf("RedactMessage: %v", err)
	}
	if v := red.BinaryFields["55.9F6B"]; strings.Trim(v, "*") != "" {
		t.Fatalf("redact leaked unknown tag: %q", v)
	}
	if !contains(paths, "55.9F6B") {
		t.Fatalf("redact must list 55.9F6B as redacted, got %v", paths)
	}

	// view (text and json) must mask the unknown tag everywhere.
	for _, format := range []string{"describe", "json"} {
		res, err := ViewMessage(raw.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), format, nil, render.NewPalette(false), false)
		if err != nil {
			t.Fatalf("ViewMessage(%s): %v", format, err)
		}
		if strings.Contains(res.Body, track2) || strings.Contains(res.Body, pan) {
			t.Fatalf("view %s leaked unknown tag bytes:\n%s", format, res.Body)
		}
		for _, ut := range res.UnknownTags {
			if strings.Trim(ut.Raw, "*") != "" {
				t.Fatalf("view %s unknown-tag list leaked %q", format, ut.Raw)
			}
		}
	}

	// validate must mask the unknown tag in its report.
	report := ValidateMessage(raw.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog(), false)
	for _, ut := range report.UnknownTags {
		if strings.Trim(ut.Raw, "*") != "" {
			t.Fatalf("validate leaked unknown tag %q", ut.Raw)
		}
	}

	// convert must still preserve the unknown tag verbatim for round-trip.
	roundTrip, err := MessageToDocument(spec.MessageSpec, raw.Raw)
	if err != nil {
		t.Fatalf("MessageToDocument: %v", err)
	}
	if roundTrip.BinaryFields["55.9F6B"] != track2 {
		t.Fatalf("convert must preserve unknown tag, got %q", roundTrip.BinaryFields["55.9F6B"])
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func TestRedactDeterministic(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	raw, err := WriteMessage(basei.AuthRequest(), spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	first, _, err := RedactMessage(spec.MessageSpec, raw.Raw)
	if err != nil {
		t.Fatalf("RedactMessage: %v", err)
	}
	second, _, err := RedactMessage(spec.MessageSpec, raw.Raw)
	if err != nil {
		t.Fatalf("RedactMessage: %v", err)
	}
	if first.Fields["2"] != second.Fields["2"] || first.Fields["35"] != second.Fields["35"] {
		t.Fatal("redaction must be deterministic")
	}
}

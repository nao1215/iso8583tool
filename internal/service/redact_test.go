package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messagespec"
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

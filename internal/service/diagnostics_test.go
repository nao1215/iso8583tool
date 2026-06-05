package service

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/messagespec"
)

// TestDiagnoseSpecAmbiguousListsTiedPresets checks that a plain-ASCII message
// fitting more than one preset surfaces all tied presets, not just the default.
func TestDiagnoseSpecAmbiguousListsTiedPresets(t *testing.T) {
	t.Parallel()

	// An 0800 without field 55 fits basei-starter and spec87ascii identically.
	doc := messageio.Document{MTI: "0800", Fields: map[string]string{"11": "123456", "70": "301"}}
	packed, err := WriteMessage(doc, basei.Spec87ASCIIWithSecondaryFields())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	diag := DiagnoseSpec(packed.Raw)
	if !diag.Ambiguous {
		t.Fatalf("expected ambiguous diagnosis, got %#v", diag)
	}
	want := map[string]bool{basei.StarterPreset: false, basei.Spec87ASCIIPreset: false}
	for _, r := range diag.Recommendations {
		if _, ok := want[r]; ok {
			want[r] = true
		}
	}
	for preset, seen := range want {
		if !seen {
			t.Errorf("ambiguous recommendations %v should include %q", diag.Recommendations, preset)
		}
	}
}

// TestDiagnoseSpecFlagsMalformedInput checks that a truncated capture no preset
// can unpack is reported as likely malformed rather than only "custom layout".
func TestDiagnoseSpecFlagsMalformedInput(t *testing.T) {
	t.Parallel()

	raw, err := hex.DecodeString("010000000000000008000103DF")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	diag := DiagnoseSpec(raw)
	if diag.Recommended != "" {
		t.Fatalf("a truncated message should have no recommendation, got %q", diag.Recommended)
	}
	if !diag.LikelyMalformed {
		t.Fatalf("a truncated message should be flagged LikelyMalformed, got %#v", diag)
	}
}

// TestValidateHintForMalformedInput checks that the validate hint points at
// corruption, not at doctor, when the message is clearly truncated.
func TestValidateHintForMalformedInput(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	raw, err := hex.DecodeString("010000000000000008000103DF")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	report := ValidateMessage(raw, spec.MessageSpec, "basei-starter", basei.DefaultExtensionCatalog(), false)
	if !strings.Contains(strings.ToLower(report.Hint), "truncat") && !strings.Contains(strings.ToLower(report.Hint), "malform") {
		t.Fatalf("hint should mention truncation/malformed input, got %q", report.Hint)
	}
	if strings.Contains(report.Hint, "doctor") {
		t.Fatalf("hint for malformed input should not steer to doctor, got %q", report.Hint)
	}
}

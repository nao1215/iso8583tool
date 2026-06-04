package service

import (
	"encoding/hex"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
)

func candidate(t *testing.T, diag SpecDiagnosis, preset string) SpecCandidate {
	t.Helper()
	for _, c := range diag.Candidates {
		if c.Preset == preset {
			return c
		}
	}
	t.Fatalf("preset %q missing from diagnosis", preset)
	return SpecCandidate{}
}

func TestDiagnoseSpecRecommendsStarterForBaseI(t *testing.T) {
	t.Parallel()

	// A BASE I message uses EMV TLV in field 55, which only the starter preset
	// can decode, so detection must be unambiguous.
	result, err := WriteMessage(basei.AuthResponse(), basei.StarterMessageSpec())
	if err != nil {
		t.Fatalf("pack sample: %v", err)
	}

	diag := DiagnoseSpec(result.Raw)
	if diag.Recommended != basei.StarterPreset {
		t.Fatalf("recommended %q, want %q", diag.Recommended, basei.StarterPreset)
	}
	if diag.Ambiguous {
		t.Error("BASE I message should not be ambiguous (field 55 is EMV TLV)")
	}
	starter := candidate(t, diag, basei.StarterPreset)
	if !starter.Unpacks || !starter.ExactRoundTrip {
		t.Errorf("starter candidate should unpack and round-trip: %+v", starter)
	}
	if starter.MTI != "0110" {
		t.Errorf("starter MTI = %q, want 0110", starter.MTI)
	}
}

func TestDiagnoseSpecDetectsPackedBCD(t *testing.T) {
	t.Parallel()

	// A kanmu-style raw-binary message: packed-BCD MTI, binary bitmap, one-byte
	// PAN length, packed-BCD numeric fields. Only the BCD preset fits.
	raw, err := hex.DecodeString("010070040000000000001040192499999999993273270000000011382204")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	diag := DiagnoseSpec(raw)
	if diag.Recommended != basei.Spec87BCDStarterPreset {
		t.Fatalf("recommended %q, want %q", diag.Recommended, basei.Spec87BCDStarterPreset)
	}
	bcd := candidate(t, diag, basei.Spec87BCDStarterPreset)
	if !bcd.ExactRoundTrip || bcd.MTI != "0100" {
		t.Errorf("bcd candidate = %+v", bcd)
	}
	// The ASCII presets must not falsely claim to fit binary bytes.
	if ascii := candidate(t, diag, basei.Spec87ASCIIPreset); ascii.Unpacks {
		t.Errorf("spec87ascii should not unpack binary message: %+v", ascii)
	}
}

func TestDiagnoseSpecFlagsAmbiguousPlainASCII(t *testing.T) {
	t.Parallel()

	// A plain ASCII message with no field 55 fits both basei-starter and
	// spec87ascii identically, so the result is ambiguous.
	doc := messageio.Document{
		MTI: "0800",
		Fields: map[string]string{
			"7":  "0604161616",
			"11": "654321",
			"70": "301",
		},
	}
	result, err := WriteMessage(doc, basei.Spec87ASCIIWithSecondaryFields())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	diag := DiagnoseSpec(result.Raw)
	if diag.Recommended != basei.StarterPreset {
		t.Fatalf("recommended %q, want default %q", diag.Recommended, basei.StarterPreset)
	}
	if !diag.Ambiguous {
		t.Error("a plain ASCII message should be flagged ambiguous")
	}
	if c := candidate(t, diag, basei.Spec87ASCIIPreset); !c.Unpacks {
		t.Errorf("spec87ascii should also fit: %+v", c)
	}
}

func TestDiagnoseSpecNoFitGivesNoRecommendation(t *testing.T) {
	t.Parallel()

	diag := DiagnoseSpec([]byte{0xFF, 0xFE, 0xFD})
	if diag.Recommended != "" {
		t.Errorf("garbage should yield no recommendation, got %q", diag.Recommended)
	}
	for _, c := range diag.Candidates {
		if c.Unpacks {
			t.Errorf("preset %q should not unpack garbage", c.Preset)
		}
	}
}

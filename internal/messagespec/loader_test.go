package messagespec

import (
	"testing"

	"github.com/nao1215/iso8583tool/internal/config"
)

func TestLoadPresets(t *testing.T) {
	t.Parallel()

	if s, err := Load(".", config.Config{}); err != nil || s.Label != "basei-starter" {
		t.Fatalf("default preset: label=%q err=%v", labelOf(s), err)
	}
	if s, err := Load(".", config.Config{Spec: "spec87ascii"}); err != nil || s.Label != "spec87ascii" {
		t.Fatalf("spec87ascii: label=%q err=%v", labelOf(s), err)
	} else if _, ok := s.MessageSpec.Fields[70]; !ok {
		t.Fatal("spec87ascii preset should include field 70")
	}
	if s, err := Load(".", config.Config{Spec: "spec87bcd-starter"}); err != nil || s.Label != "spec87bcd-starter" {
		t.Fatalf("spec87bcd-starter: label=%q err=%v", labelOf(s), err)
	} else if s.MessageSpec.Name != "ISO 8583:1987 Packed BCD Starter" {
		t.Fatalf("packed BCD preset: name=%q", s.MessageSpec.Name)
	}
	// A non-preset value is treated as a path; a non-JSON extension is rejected.
	if _, err := Load(".", config.Config{Spec: "not-a-preset"}); err == nil {
		t.Fatal("expected error for a non-preset, non-json spec")
	}
	if _, err := Load(".", config.Config{Spec: "spec.yaml"}); err == nil {
		t.Fatal("expected json-only error")
	}
	if _, err := Load(t.TempDir(), config.Config{Spec: "missing.json"}); err == nil {
		t.Fatal("expected a file-not-found error")
	}
}

func labelOf(s *Spec) string {
	if s == nil {
		return ""
	}
	return s.Label
}

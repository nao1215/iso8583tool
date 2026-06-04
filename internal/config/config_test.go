package config

import "testing"

func TestParseRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := Default("demo")
	cfg.Spec.MessageSpec = "./specs/basei.json"
	cfg.Transport.Header = "ascii4"

	parsed, err := Parse([]byte(cfg.String()))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if parsed.Project.Name != "demo" {
		t.Fatalf("Project.Name = %q, want demo", parsed.Project.Name)
	}
	if parsed.Spec.Preset != "basei-starter" {
		t.Fatalf("Spec.Preset = %q, want basei-starter", parsed.Spec.Preset)
	}
	if parsed.Spec.MessageSpec != "./specs/basei.json" {
		t.Fatalf("Spec.MessageSpec = %q, want ./specs/basei.json", parsed.Spec.MessageSpec)
	}
	if parsed.Transport.Header != "ascii4" {
		t.Fatalf("Transport.Header = %q, want ascii4", parsed.Transport.Header)
	}
}

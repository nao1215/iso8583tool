package basei

import "testing"

func TestPresetsAreConsistent(t *testing.T) {
	t.Parallel()

	presets := Presets()
	if len(presets) == 0 {
		t.Fatal("Presets() returned no presets")
	}

	defaults := 0
	seen := map[string]bool{}
	for _, p := range presets {
		if seen[p.Name] {
			t.Errorf("duplicate preset name %q", p.Name)
		}
		seen[p.Name] = true

		if p.Title == "" || p.Encoding == "" || p.Summary == "" {
			t.Errorf("preset %q has empty metadata", p.Name)
		}
		if p.Default {
			defaults++
		}
		if p.Spec() == nil {
			t.Errorf("preset %q built a nil spec", p.Name)
		}
	}

	if defaults != 1 {
		t.Errorf("want exactly one default preset, got %d", defaults)
	}
}

func TestPresetsCoverKnownNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{StarterPreset, Spec87ASCIIPreset, Spec87BCDStarterPreset} {
		p, ok := LookupPreset(name)
		if !ok {
			t.Fatalf("LookupPreset(%q) not found", name)
		}
		if p.Name != name {
			t.Errorf("LookupPreset(%q) returned name %q", name, p.Name)
		}
	}

	if _, ok := LookupPreset("not-a-preset.json"); ok {
		t.Error("LookupPreset matched a non-preset name")
	}
}

func TestDefaultPresetIsStarter(t *testing.T) {
	t.Parallel()

	for _, p := range Presets() {
		if p.Default && p.Name != StarterPreset {
			t.Errorf("default preset is %q, want %q", p.Name, StarterPreset)
		}
	}
}

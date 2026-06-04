package render

import "testing"

func TestPalette(t *testing.T) {
	t.Parallel()

	on := NewPalette(true)
	if !on.Enabled() {
		t.Fatal("palette should be enabled")
	}
	if on.Red("x") == "x" {
		t.Fatal("enabled palette should wrap with escapes")
	}

	off := NewPalette(false)
	if off.Enabled() {
		t.Fatal("palette should be disabled")
	}
	if off.Red("x") != "x" || off.BoldCyan("y") != "y" {
		t.Fatal("disabled palette should not wrap")
	}
}

func TestResolveColor(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"always": true,
		"never":  false,
		"auto":   false, // nil file is not a terminal
		"bogus":  false,
	}
	for mode, want := range cases {
		if got := ResolveColor(mode, nil); got != want {
			t.Errorf("ResolveColor(%q, nil) = %v, want %v", mode, got, want)
		}
	}
}

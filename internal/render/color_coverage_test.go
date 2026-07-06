package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPaletteAllHelpers exercises every color helper in both enabled and
// disabled states so no helper (e.g. Blue for the "positional" strategy or
// Magenta for "bitmap") goes unverified.
func TestPaletteAllHelpers(t *testing.T) {
	t.Parallel()

	helpers := map[string]func(Palette, string) string{
		"Bold":      Palette.Bold,
		"Dim":       Palette.Dim,
		"Red":       Palette.Red,
		"Green":     Palette.Green,
		"Yellow":    Palette.Yellow,
		"Blue":      Palette.Blue,
		"Magenta":   Palette.Magenta,
		"Cyan":      Palette.Cyan,
		"BoldCyan":  Palette.BoldCyan,
		"BoldGreen": Palette.BoldGreen,
	}
	on := NewPalette(true)
	off := NewPalette(false)
	for name, fn := range helpers {
		got := fn(on, "x")
		if !strings.HasPrefix(got, "\x1b[") || !strings.HasSuffix(got, "\x1b[0m") {
			t.Errorf("%s enabled should wrap in ANSI escapes, got %q", name, got)
		}
		if fn(off, "x") != "x" {
			t.Errorf("%s disabled should return the string unchanged", name)
		}
		// An empty string is never wrapped, even when enabled.
		if fn(on, "") != "" {
			t.Errorf("%s should not wrap an empty string", name)
		}
	}
}

// TestResolveColorNoColor covers the NO_COLOR short-circuit: with NO_COLOR set,
// auto never enables color even when writing to a terminal.
func TestResolveColorNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if ResolveColor("auto", os.Stdout) {
		t.Fatal("auto with NO_COLOR set must disable color")
	}
	// "always" ignores NO_COLOR (explicit override wins).
	if !ResolveColor("always", os.Stdout) {
		t.Fatal("always must enable color regardless of NO_COLOR")
	}
}

// TestIsTerminal covers the nil, stat-success-non-tty, and closed-file paths.
func TestIsTerminal(t *testing.T) {
	t.Parallel()

	if IsTerminal(nil) {
		t.Fatal("nil file is not a terminal")
	}
	// A regular file is not a character device.
	path := filepath.Join(t.TempDir(), "regular.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path) //nolint:gosec // path is a controlled test temp file
	if err != nil {
		t.Fatal(err)
	}
	if IsTerminal(f) {
		t.Fatal("a regular file is not a terminal")
	}
	// Stat on a closed file returns an error, exercising the error branch.
	_ = f.Close()
	if IsTerminal(f) {
		t.Fatal("a closed file is not a terminal")
	}
}

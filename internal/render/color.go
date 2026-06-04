// Package render provides a tiny, dependency-free ANSI color palette and the
// rules for deciding when color should be emitted.
package render

import (
	"os"
	"strings"
)

// Palette wraps strings in ANSI escape codes when enabled. The zero value is a
// disabled palette, so every helper is safe to call unconditionally.
type Palette struct {
	enabled bool
}

// NewPalette returns a palette that emits color only when enabled is true.
func NewPalette(enabled bool) Palette {
	return Palette{enabled: enabled}
}

// Enabled reports whether the palette emits color.
func (p Palette) Enabled() bool { return p.enabled }

func (p Palette) wrap(code, s string) string {
	if !p.enabled || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// Color helpers used across the CLI output.
func (p Palette) Bold(s string) string    { return p.wrap("1", s) }
func (p Palette) Dim(s string) string     { return p.wrap("2", s) }
func (p Palette) Red(s string) string     { return p.wrap("31", s) }
func (p Palette) Green(s string) string   { return p.wrap("32", s) }
func (p Palette) Yellow(s string) string  { return p.wrap("33", s) }
func (p Palette) Blue(s string) string    { return p.wrap("34", s) }
func (p Palette) Magenta(s string) string { return p.wrap("35", s) }
func (p Palette) Cyan(s string) string    { return p.wrap("36", s) }

// BoldCyan and friends compose two attributes for headers.
func (p Palette) BoldCyan(s string) string  { return p.wrap("1;36", s) }
func (p Palette) BoldGreen(s string) string { return p.wrap("1;32", s) }

// ResolveColor turns a --color flag value into a concrete on/off decision.
// mode is one of "auto", "always", "never". For "auto", color is enabled only
// when NO_COLOR is unset and out refers to a terminal (character device).
func ResolveColor(mode string, out *os.File) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "always", "force", "yes":
		return true
	case "never", "off", "no":
		return false
	case "", "auto":
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			return false
		}
		return IsTerminal(out)
	default:
		return false
	}
}

// IsTerminal reports whether f refers to a terminal (character device).
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

package render

import (
	"strings"
	"testing"
)

// FuzzSanitizeControl ensures the sanitizer never panics, never emits a raw
// control byte, and is idempotent — sanitizing already-sanitized text is a
// no-op, so the output is a stable, terminal-safe form.
func FuzzSanitizeControl(f *testing.F) {
	for _, seed := range []string{"hello", "\x1b[2J", "0100 · STAN", "\x00\x07\x7f", "a\x1bb", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := SanitizeControl(s)
		// No raw control byte may survive in the output.
		if i := strings.IndexFunc(out, isControlRune); i >= 0 {
			t.Fatalf("sanitized output still contains a control byte at %d: %q", i, out)
		}
		// Idempotent: sanitizing sanitized text changes nothing.
		if again := SanitizeControl(out); again != out {
			t.Fatalf("SanitizeControl is not idempotent: %q -> %q -> %q", s, out, again)
		}
		// A string with no control bytes is returned unchanged.
		if strings.IndexFunc(s, isControlRune) < 0 && out != s {
			t.Fatalf("clean string was altered: %q -> %q", s, out)
		}
	})
}

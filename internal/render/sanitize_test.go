package render

import "testing"

func TestSanitizeControl(t *testing.T) {
	t.Parallel()

	esc := "\x1b[2J"
	cases := []struct{ name, in, want string }{
		{"plain text unchanged", "HELLO 123", "HELLO 123"},
		{"utf8 separator kept", "0100 · STAN 123456", "0100 · STAN 123456"},
		{"esc clear screen", esc, "^[[2J"},
		{"nul", "\x00", "^@"},
		{"del", "\x7f", "^?"},
		{"mixed", "A\x1bB", "A^[B"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := SanitizeControl(tc.in); got != tc.want {
				t.Fatalf("SanitizeControl(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	// The output must never contain a raw ESC byte.
	for _, r := range SanitizeControl("\x1b\x07\x00") {
		if r == 0x1b || r == 0x07 || r == 0x00 {
			t.Fatalf("sanitized output still contains a raw control byte: %q", r)
		}
	}
}

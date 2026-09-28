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
		{"newline and tab", "A\nB\tC\r", "A^JB^IC^M"},
		{"c1 csi rune", "A\u009b2J", "AM-^[2J"},
		{"c1 nel rune", "\u0085", "M-^E"},
		{"lone csi byte", "A\x9b2J", "AM-^[2J"},
		{"invalid utf8 byte", "\xffA\xe9", "M-^?AM-i"},
		{"truncated utf8 sequence", "\xc3", "M-C"},
		{"printable latin1 rune kept", "caf\u00e9 \u00a0", "caf\u00e9 \u00a0"},
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

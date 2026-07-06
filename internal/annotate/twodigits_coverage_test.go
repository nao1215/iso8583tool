package annotate

import "testing"

// TestTwoDigits covers both branches of twoDigits: a value of length >= 2 is
// truncated to its first two runes, and a shorter value is returned as-is
// (after trimming surrounding whitespace).
func TestTwoDigits(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"1234", "12"},
		{"  56 ", "56"},
		{"7", "7"},
		{"", ""},
		{" 9", "9"},
	}
	for _, c := range cases {
		if got := twoDigits(c.in); got != c.want {
			t.Errorf("twoDigits(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

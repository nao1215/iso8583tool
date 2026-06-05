package render

import "strings"

// SanitizeControl replaces control characters in s with a visible caret form,
// like `cat -v`: ESC becomes "^[", NUL "^@", DEL "^?", and so on. A field value
// carrying raw ANSI/control bytes can otherwise move the cursor, clear the
// screen, or recolor the terminal when printed in a text view. Printable text,
// including multi-byte UTF-8 such as the "·" summary separator, is left
// unchanged, and a value with no control bytes is returned as-is.
func SanitizeControl(s string) string {
	if !strings.ContainsFunc(s, isControlRune) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		if isControlRune(r) {
			b.WriteString(caretNotation(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isControlRune reports whether r is an ASCII control character (including DEL),
// which must not be emitted verbatim to a terminal.
func isControlRune(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// caretNotation renders a control rune in caret form: 0x00-0x1f map to "^@".."^_"
// and 0x7f (DEL) maps to "^?".
func caretNotation(r rune) string {
	if r == 0x7f {
		return "^?"
	}
	return "^" + string(r+0x40)
}

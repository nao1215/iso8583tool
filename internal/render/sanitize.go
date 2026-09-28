package render

import (
	"strings"
	"unicode/utf8"
)

// SanitizeControl replaces the characters a terminal would act on with a
// visible form, like `cat -v`: a C0 control or DEL becomes caret notation (ESC
// "^[", NUL "^@", newline "^J", DEL "^?"), and a C1 control (U+0080-U+009F,
// where U+009B is CSI) or a byte that is not valid UTF-8 becomes "M-" plus the
// low seven bits in the same notation (CSI "M-^[", 0xFF "M-^?", 0xE9 "M-i"). A
// field value carrying raw ANSI/control bytes can otherwise move the cursor,
// clear the screen, or recolor the terminal when printed in a text view, and an
// 8-bit terminal reads a lone 0x9B byte as CSI. Printable text, including
// multi-byte UTF-8 such as the "·" summary separator, is left unchanged, and a
// value with nothing to escape is returned as-is.
func SanitizeControl(s string) string {
	if utf8.ValidString(s) && !strings.ContainsFunc(s, isControlRune) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteString(metaNotation(s[i]))
		case isControlRune(r) && size == 1: // C0 or DEL: the rune is the byte
			b.WriteString(caretNotation(s[i]))
		case isControlRune(r): // C1 U+0080-U+009F is encoded C2 80..C2 9F
			b.WriteString(metaNotation(s[i+1]))
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// isControlRune reports whether r is a C0 control character, DEL, or a C1
// control character, none of which may be emitted verbatim to a terminal.
func isControlRune(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f)
}

// caretNotation renders a 7-bit byte that is a control in caret form (0x00-0x1f
// map to "^@".."^_", 0x7f to "^?") and any other 7-bit byte as itself.
func caretNotation(c byte) string {
	switch {
	case c == 0x7f:
		return "^?"
	case c < 0x20:
		return "^" + string(rune(c+0x40))
	default:
		return string(rune(c))
	}
}

// metaNotation renders a byte with the high bit set the way `cat -v` does: "M-"
// followed by the caret form of its low seven bits.
func metaNotation(c byte) string {
	return "M-" + caretNotation(c&0x7f)
}

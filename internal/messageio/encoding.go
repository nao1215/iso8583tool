package messageio

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// MaxInputSize caps how many bytes a single message source (file, stdin, or an
// inline --raw value) may contribute. A single ISO 8583 message is at most a few
// kilobytes even when hex-encoded, so 1 MiB is a generous ceiling that still
// rejects oversized or runaway input before it can exhaust memory.
const MaxInputSize = 1 << 20 // 1 MiB

// errInputTooLarge is returned when a source exceeds MaxInputSize.
var errInputTooLarge = fmt.Errorf("input exceeds the %d-byte limit", MaxInputSize)

// ReadSource returns the raw bytes for a target without decoding. The target
// may be a file path, "-" (stdin), or "" (stdin). inline, when non-empty, is
// used verbatim instead. stdin may be nil when not available. Any single source
// is capped at MaxInputSize so oversized input fails cleanly instead of being
// loaded in full.
func ReadSource(target, inline string, stdin io.Reader) ([]byte, error) {
	if strings.TrimSpace(inline) != "" {
		if strings.TrimSpace(target) != "" {
			return nil, errors.New("provide either a file argument or --raw, not both")
		}
		if len(inline) > MaxInputSize {
			return nil, errInputTooLarge
		}
		return []byte(inline), nil
	}
	if target == "-" || strings.TrimSpace(target) == "" {
		if stdin == nil {
			return nil, errors.New("provide a message file argument, --raw, or pipe input via stdin")
		}
		return readLimited(stdin)
	}
	f, err := os.Open(filepath.Clean(target))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return readLimited(f)
}

// readLimited reads from r up to MaxInputSize bytes and reports an error if the
// source has more data, so a giant file or stdin stream cannot be slurped whole.
func readLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxInputSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxInputSize {
		return nil, errInputTooLarge
	}
	return data, nil
}

// ReadMessage reads a message from a file, stdin, or inline value and decodes
// it with the given encoding.
func ReadMessage(target, inline, encoding string, stdin io.Reader) ([]byte, error) {
	data, err := ReadSource(target, inline, stdin)
	if err != nil {
		return nil, err
	}
	return DecodeInput(data, encoding)
}

// LooksLikeHex reports whether data is plausibly a hex-encoded message: after
// dropping ASCII whitespace it is non-empty, has an even number of digits, and
// contains only hex digits. Raw binary messages carry control bytes (such as a
// binary bitmap), so they fail this test — which makes it a reliable way to
// auto-pick the input encoding when the caller does not know it.
func LooksLikeHex(data []byte) bool {
	n := 0
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			continue
		}
		if !isHexDigit(b) {
			return false
		}
		n++
	}
	return n > 0 && n%2 == 0
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func DecodeInput(data []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "hex":
		clean := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, string(data))
		if clean == "" {
			return nil, errors.New("hex input is empty")
		}
		decoded, err := hex.DecodeString(clean)
		if err != nil {
			return nil, fmt.Errorf("decode hex input: %w; if this is a raw binary message, pass --encoding raw", err)
		}
		return decoded, nil
	case "raw":
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
}

func EncodeOutput(raw []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "hex":
		buf := make([]byte, hex.EncodedLen(len(raw)))
		hex.Encode(buf, raw)
		return []byte(strings.ToUpper(string(buf))), nil
	case "raw":
		return raw, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
}

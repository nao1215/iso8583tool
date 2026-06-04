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

// ReadSource returns the raw bytes for a target without decoding. The target
// may be a file path, "-" (stdin), or "" (stdin). inline, when non-empty, is
// used verbatim instead. stdin may be nil when not available.
func ReadSource(target, inline string, stdin io.Reader) ([]byte, error) {
	if strings.TrimSpace(inline) != "" {
		if strings.TrimSpace(target) != "" {
			return nil, errors.New("provide either a file argument or --raw, not both")
		}
		return []byte(inline), nil
	}
	if target == "-" || strings.TrimSpace(target) == "" {
		if stdin == nil {
			return nil, errors.New("provide a message file argument, --raw, or pipe input via stdin")
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	return os.ReadFile(filepath.Clean(target))
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
			return nil, fmt.Errorf("decode hex input: %w", err)
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

package messageio

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func ReadRawInput(filePath, raw, encoding string) ([]byte, error) {
	hasFile := strings.TrimSpace(filePath) != ""
	hasRaw := strings.TrimSpace(raw) != ""
	if hasFile == hasRaw {
		return nil, errors.New("provide exactly one of --file or --raw")
	}

	var data []byte
	var err error
	if hasFile {
		data, err = os.ReadFile(filepath.Clean(filePath))
		if err != nil {
			return nil, err
		}
	} else {
		data = []byte(raw)
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

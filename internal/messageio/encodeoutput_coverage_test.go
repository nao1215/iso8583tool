package messageio

import "testing"

// TestEncodeOutput covers the hex, raw, and unsupported-encoding branches.
func TestEncodeOutput(t *testing.T) {
	t.Parallel()

	hexOut, err := EncodeOutput([]byte{0xAB, 0xCD}, "hex")
	if err != nil || string(hexOut) != "ABCD" {
		t.Errorf("EncodeOutput hex = %q,%v", hexOut, err)
	}

	defaultOut, err := EncodeOutput([]byte{0x01}, "")
	if err != nil || string(defaultOut) != "01" {
		t.Errorf("EncodeOutput default(hex) = %q,%v", defaultOut, err)
	}

	rawOut, err := EncodeOutput([]byte("raw"), "raw")
	if err != nil || string(rawOut) != "raw" {
		t.Errorf("EncodeOutput raw = %q,%v", rawOut, err)
	}

	if _, err := EncodeOutput([]byte{0x01}, "base64"); err == nil {
		t.Error("EncodeOutput with an unsupported encoding should error")
	}
}

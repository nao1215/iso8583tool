package messageio

import (
	"strings"
	"testing"
)

// TestValidateMTIFormat locks in that a document's MTI must be exactly four
// decimal digits, so a malformed MTI fails with a plain message at parse time
// instead of leaking moov's pack-time internals.
func TestValidateMTIFormat(t *testing.T) {
	t.Parallel()

	valid := []string{"0100", "0810", "9999", "0000"}
	for _, mti := range valid {
		doc := Document{MTI: mti, Fields: map[string]string{"11": "000001"}}
		if err := doc.Validate(); err != nil {
			t.Errorf("MTI %q should be valid, got %v", mti, err)
		}
	}

	invalid := []string{"ABCD", "01", "010", "01000", "01a0", "0x10", " 010", "01 0"}
	for _, mti := range invalid {
		doc := Document{MTI: mti, Fields: map[string]string{"11": "000001"}}
		err := doc.Validate()
		if err == nil {
			t.Errorf("MTI %q should be rejected", mti)
			continue
		}
		if got := err.Error(); !strings.Contains(got, "4 digits") {
			t.Errorf("MTI %q error should mention '4 digits', got %q", mti, got)
		}
	}

	// An empty MTI keeps its own dedicated message (not the digit message).
	if err := (Document{MTI: ""}).Validate(); err == nil || !strings.Contains(err.Error(), "requires mti") {
		t.Errorf("empty MTI should report 'requires mti', got %v", err)
	}
}

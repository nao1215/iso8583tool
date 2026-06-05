package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
)

// TestBinaryFieldsRejectedForTextFields guards that an ASCII text field cannot be
// poisoned with raw control bytes through binary_fields: it must be set via
// fields. A genuinely binary field (the PIN, 52) is still accepted.
func TestBinaryFieldsRejectedForTextFields(t *testing.T) {
	t.Parallel()

	spec := basei.StarterMessageSpec()

	// Every ASCII text field here must reject a binary_fields entry.
	for _, id := range []string{"11", "39", "41", "48", "49", "63", "100"} {
		doc := messageio.Document{MTI: "0100", BinaryFields: map[string]string{id: "000102030405"}}
		_, err := WriteMessage(doc, spec)
		if err == nil {
			t.Errorf("field %s is a text field; a binary_fields entry must be rejected", id)
			continue
		}
		if !strings.Contains(err.Error(), "text field") {
			t.Errorf("field %s rejection should explain it is a text field, got %v", id, err)
		}
	}

	// A text field set through "fields" is fine.
	if _, err := WriteMessage(messageio.Document{MTI: "0100", Fields: map[string]string{"11": "123456"}}, spec); err != nil {
		t.Errorf("setting a text field via fields should work: %v", err)
	}
}

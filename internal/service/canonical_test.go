package service

import (
	"strings"
	"testing"

	"github.com/moov-io/iso8583"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messagespec"
	"github.com/nao1215/iso8583tool/internal/render"
)

// TestDecodeFieldsCanonical checks that decoded[].value keeps the same canonical
// (zero-padded) width as the document's fields map, instead of the collapsed
// integer form field.String() returns for a padded fixed-length field.
func TestDecodeFieldsCanonical(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	packed, err := WriteMessage(basei.AuthRequest(), spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	msg := iso8583.NewMessage(spec.MessageSpec)
	if err := msg.Unpack(packed.Raw); err != nil {
		t.Fatalf("Unpack: %v", err)
	}

	got := map[string]string{}
	for _, d := range DecodeFields(msg) {
		got[d.Path] = d.Value
	}
	if got["3"] != "000000" {
		t.Errorf("decoded F3 value = %q, want canonical %q", got["3"], "000000")
	}
}

// TestViewDescribeCanonical checks that the full describe output keeps the
// canonical width of zero-padded fixed-length fields, matching the filtered and
// JSON views.
func TestViewDescribeCanonical(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	packed, err := WriteMessage(basei.AuthRequest(), spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	result, err := ViewMessage(packed.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false), false)
	if err != nil {
		t.Fatalf("ViewMessage: %v", err)
	}
	// Inspect the F3 / F4 top-level lines specifically: the same canonical value
	// can also appear in an EMV subfield (55.9F02), so a plain Contains check is
	// not enough to prove the top-level lines are canonical.
	for id, want := range map[string]string{"3": "000000", "4": "000000005000"} {
		if got := describeLineValue(result.Body, "F"+id+" "); got != want {
			t.Errorf("describe F%s value = %q, want canonical %q\n%s", id, got, want, result.Body)
		}
	}
}

// describeLineValue returns the value (text after the last ": ") of the first
// describe line that starts with the given prefix, or "" when none matches.
func describeLineValue(body, prefix string) string {
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		idx := strings.LastIndex(line, ": ")
		if idx < 0 {
			return ""
		}
		// Drop any trailing "  → meaning" annotation.
		value := line[idx+2:]
		if cut := strings.Index(value, "  "); cut >= 0 {
			value = value[:cut]
		}
		return strings.TrimSpace(value)
	}
	return ""
}

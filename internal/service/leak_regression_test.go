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

func regressionSpec(t *testing.T) *messagespec.Spec {
	t.Helper()
	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	return spec
}

// TestPINFieldIsHexAndMasked is a regression for a bug found by FuzzConvertRoundTrip:
// field 52 (PIN, a binary-encoded primitive) was emitted into the text `fields`
// map, which put raw control bytes in the JSON and — worse — meant redact and
// view never masked the PIN block (sensitive authentication data). It must now
// be hex in binary_fields and masked everywhere it is displayed.
func TestPINFieldIsHexAndMasked(t *testing.T) {
	t.Parallel()
	spec := regressionSpec(t)

	const pin = "0123456789ABCDEF"
	msg := iso8583.NewMessage(spec.MessageSpec)
	msg.MTI("0100")
	if err := msg.Field(2, "4111111111111111"); err != nil {
		t.Fatalf("set F2: %v", err)
	}
	if err := msg.Field(11, "123456"); err != nil {
		t.Fatalf("set F11: %v", err)
	}
	if err := msg.Field(52, pin); err != nil {
		t.Fatalf("set F52: %v", err)
	}
	raw, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	doc, err := MessageToDocument(spec.MessageSpec, raw)
	if err != nil {
		t.Fatalf("MessageToDocument: %v", err)
	}
	if _, inText := doc.Fields["52"]; inText {
		t.Fatalf("F52 must not be a text field (raw bytes leak into JSON): %#v", doc.Fields)
	}
	if _, inBinary := doc.BinaryFields["52"]; !inBinary {
		t.Fatalf("F52 should be emitted as a binary (hex) field, got %#v", doc.BinaryFields)
	}

	// redact must mask the PIN.
	red, _, err := RedactMessage(spec.MessageSpec, raw)
	if err != nil {
		t.Fatalf("RedactMessage: %v", err)
	}
	if v := red.BinaryFields["52"]; strings.Trim(v, "*") != "" {
		t.Fatalf("redact left the PIN unmasked: %q", v)
	}

	// view (json and describe) must never print the PIN.
	for _, format := range []string{"json", "describe"} {
		res, err := ViewMessage(raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), format, nil, render.NewPalette(false))
		if err != nil {
			t.Fatalf("ViewMessage(%s): %v", format, err)
		}
		if strings.Contains(res.Body, pin) {
			t.Fatalf("view %s leaked the PIN block:\n%s", format, res.Body)
		}
	}
}

// TestViewDescribeMasksTrackData is a regression for a bug found by
// FuzzViewNeverLeaksPAN: the text/describe view relied on moov's Track filters,
// which pass a value through unchanged when it is not parseable track data (and
// in the basei-starter spec the track fields are plain strings, so they never
// parse). Full track data — including expiry and discretionary data — leaked.
func TestViewDescribeMasksTrackData(t *testing.T) {
	t.Parallel()
	spec := regressionSpec(t)

	const track2 = "4111111111111111D29122011234567890" // PAN + expiry + discretionary
	const track1 = "A0000000000000000000000000000000"   // not well-formed track1

	msg := iso8583.NewMessage(spec.MessageSpec)
	msg.MTI("0100")
	if err := msg.Field(11, "123456"); err != nil {
		t.Fatalf("set F11: %v", err)
	}
	if err := msg.Field(35, track2); err != nil {
		t.Fatalf("set F35: %v", err)
	}
	if err := msg.Field(45, track1); err != nil {
		t.Fatalf("set F45: %v", err)
	}
	raw, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	res, err := ViewMessage(raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false))
	if err != nil {
		t.Fatalf("ViewMessage: %v", err)
	}
	for _, secret := range []string{track2, track1} {
		if strings.Contains(res.Body, secret) {
			t.Fatalf("view describe leaked track data %q:\n%s", secret, res.Body)
		}
	}
	// The expiry digits that follow the PAN in track 2 must not survive either.
	if strings.Contains(res.Body, "D29122011234567890") {
		t.Fatalf("view describe leaked track 2 expiry/discretionary:\n%s", res.Body)
	}
}

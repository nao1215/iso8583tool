package service

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"
	moovsort "github.com/moov-io/iso8583/sort"
	moovspecs "github.com/moov-io/iso8583/specs"

	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/messagespec"
)

// loadSpecFile writes a JSON spec to a temp file and loads it into a moov spec.
func loadSpecFile(t *testing.T, name, body string) *iso8583.MessageSpec {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	s, err := messagespec.Load(dir, config.Config{Spec: path})
	if err != nil {
		t.Fatalf("load spec %s: %v", name, err)
	}
	return s.MessageSpec
}

// positionalCompositeSpec returns a spec whose field 3 is a positional composite
// (3.1 plus a nested positional composite 3.2.1), not a BER-TLV one.
func positionalCompositeSpec() *iso8583.MessageSpec {
	fields := maps.Clone(moovspecs.Spec87ASCII.Fields)
	fields[3] = field.NewComposite(&field.Spec{
		Length:      8,
		Description: "Positional composite",
		Pref:        prefix.ASCII.Fixed,
		Tag:         &field.TagSpec{Sort: moovsort.StringsByInt}, // no Enc => positional
		Subfields: map[string]field.Field{
			"1": field.NewString(&field.Spec{Length: 2, Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
			"2": field.NewComposite(&field.Spec{
				Length: 6,
				Pref:   prefix.ASCII.Fixed,
				Tag:    &field.TagSpec{Sort: moovsort.StringsByInt},
				Subfields: map[string]field.Field{
					"1": field.NewString(&field.Spec{Length: 6, Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
				},
			}),
		},
	})
	return &iso8583.MessageSpec{Name: "positional-composite", Fields: fields}
}

func TestMessageToDocumentExpandsPositionalComposite(t *testing.T) {
	t.Parallel()

	spec := positionalCompositeSpec()
	msg := iso8583.NewMessage(spec)
	msg.MTI("0100")
	if err := msg.MarshalPath("3.1", "00"); err != nil {
		t.Fatalf("set 3.1: %v", err)
	}
	if err := msg.MarshalPath("3.2.1", "260604"); err != nil {
		t.Fatalf("set 3.2.1: %v", err)
	}
	raw, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	doc, err := MessageToDocument(spec, raw)
	if err != nil {
		t.Fatalf("MessageToDocument: %v", err)
	}

	// Positional/nested composites expand into text child paths, and are NOT
	// hex-encoded or collapsed as if they were TLV.
	if doc.Fields["3.1"] != "00" {
		t.Fatalf("3.1 = %q, want \"00\" (fields=%v binary=%v)", doc.Fields["3.1"], doc.Fields, doc.BinaryFields)
	}
	if doc.Fields["3.2.1"] != "260604" {
		t.Fatalf("3.2.1 = %q, want \"260604\"", doc.Fields["3.2.1"])
	}
	if _, ok := doc.BinaryFields["3.1"]; ok {
		t.Fatalf("3.1 must not be hex-encoded, got %q", doc.BinaryFields["3.1"])
	}
	if _, ok := doc.BinaryFields["3.2"]; ok {
		t.Fatalf("nested composite 3.2 must be expanded, not collapsed to %q", doc.BinaryFields["3.2"])
	}

	// And it round-trips.
	back, err := WriteMessage(doc, spec)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	if string(back.Raw) != string(raw) {
		t.Fatal("positional composite did not round-trip")
	}
}

// TestNestedTLVExpandsToLeafPath checks that a constructed (nested) TLV composite
// is unpacked to its leaf dot-path (55.70.9F02) instead of being flattened into
// the parent tag's raw blob (55.70).
func TestNestedTLVExpandsToLeafPath(t *testing.T) {
	t.Parallel()

	spec := loadSpecFile(t, "55-constructed.json", `{
  "name": "Constructed TLV",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "11": {"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},
    "55": {
      "type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL",
      "tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},
      "subfields": {
        "70": {
          "type":"Composite","length":255,"description":"Template","prefix":"BerTLV",
          "tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},
          "subfields": {"9F02": {"type":"Binary","length":6,"description":"Amount","enc":"Binary","prefix":"BerTLV"}}
        }
      }
    }
  }
}`)

	doc := messageio.Document{MTI: "0110", Fields: map[string]string{"11": "123456"}, BinaryFields: map[string]string{"55.70.9F02": "000000005000"}}
	packed, err := WriteMessage(doc, spec)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	back, err := MessageToDocument(spec, packed.Raw)
	if err != nil {
		t.Fatalf("MessageToDocument: %v", err)
	}
	if got, ok := back.BinaryFields["55.70.9F02"]; !ok || got != "000000005000" {
		t.Fatalf("expected leaf path 55.70.9F02=000000005000, got %#v", back.BinaryFields)
	}
	if _, ok := back.BinaryFields["55.70"]; ok {
		t.Fatalf("nested TLV must not be flattened to 55.70: %#v", back.BinaryFields)
	}
}

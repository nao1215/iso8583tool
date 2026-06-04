package service

import (
	"maps"
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"
	moovsort "github.com/moov-io/iso8583/sort"
	moovspecs "github.com/moov-io/iso8583/specs"
)

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

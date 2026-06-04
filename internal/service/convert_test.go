package service

import (
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messagespec"
)

func TestConvertRoundTrip(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load returned error: %v", err)
	}

	// document -> packed message
	packed, err := WriteMessage(basei.AuthRequest(), spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	// packed message -> document
	doc, err := MessageToDocument(spec.MessageSpec, packed.Raw)
	if err != nil {
		t.Fatalf("MessageToDocument returned error: %v", err)
	}
	if doc.MTI != "0100" {
		t.Fatalf("doc.MTI = %q, want 0100", doc.MTI)
	}

	// document -> packed message again must reproduce the same bytes
	repacked, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage (round-trip) returned error: %v", err)
	}
	if string(repacked.Raw) != string(packed.Raw) {
		t.Fatal("convert round-trip changed the packed bytes")
	}

	// Field 55 (composite) is emitted per tag so individual EMV tags are editable.
	if _, ok := doc.BinaryFields["55.9F02"]; !ok {
		t.Fatalf("expected field 55.9F02 in binary_fields, got %#v", doc.BinaryFields)
	}
}

func TestConvertUnknownTagEditable(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load returned error: %v", err)
	}

	// A message carrying a known and an unknown EMV tag.
	doc := basei.AuthRequest()
	doc.BinaryFields["55.DF8129"] = "AABBCCDD"
	packed, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	// Unpacking back to a document must expose the unknown tag per-tag so it
	// can be edited, and re-packing must reproduce the same bytes.
	back, err := MessageToDocument(spec.MessageSpec, packed.Raw)
	if err != nil {
		t.Fatalf("MessageToDocument returned error: %v", err)
	}
	if back.BinaryFields["55.DF8129"] != "AABBCCDD" {
		t.Fatalf("unknown tag 55.DF8129 not preserved per-tag: %#v", back.BinaryFields)
	}
	repacked, err := WriteMessage(back, spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage (round-trip) returned error: %v", err)
	}
	if string(repacked.Raw) != string(packed.Raw) {
		t.Fatal("round-trip with an unknown tag changed the packed bytes")
	}
}

package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/render"
)

const nested8ASpec = `{
  "name": "Constructed TLV 8A",
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
          "subfields": {
            "8A": {"type":"Binary","length":2,"description":"ARC","enc":"Binary","prefix":"BerTLV"},
            "9A": {"type":"Binary","length":3,"description":"Txn Date","enc":"Binary","prefix":"BerTLV"}
          }
        }
      }
    }
  }
}`

// TestDescribeKeepsNestedTLVPath guards the rendering regression where the full
// describe output lost a constructed TLV's parent path: the leaf showed up as
// "70.9F02" and the nested header as "F70" instead of "F55.70". With the path
// kept, a copy-pasted dot-path works with --filter and in an edited document.
func TestDescribeKeepsNestedTLVPath(t *testing.T) {
	t.Parallel()

	spec := loadSpecFile(t, "55-8a.json", nested8ASpec)
	doc := messageio.Document{
		MTI:    "0110",
		Fields: map[string]string{"11": "123456"},
		BinaryFields: map[string]string{
			"55.70.8A": "3030",
			"55.70.9A": "260605",
		},
	}
	packed, err := WriteMessage(doc, spec)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	res, err := ViewMessage(packed.Raw, spec, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false), false)
	if err != nil {
		t.Fatalf("ViewMessage describe: %v", err)
	}
	for _, want := range []string{
		"F55.70 Template SUBFIELDS:", // nested composite header carries the full path
		"55.70.8A",                   // leaf carries the full path...
		"Approved",                   // ...and its known meaning is annotated
		"55.70.9A",
		"2026-06-05",
	} {
		if !strings.Contains(res.Body, want) {
			t.Errorf("describe output missing %q\n%s", want, res.Body)
		}
	}
	// The bug printed the leaf with only its immediate parent; that must not leak.
	if strings.Contains(res.Body, "70.8A") && !strings.Contains(res.Body, "55.70.8A") {
		t.Errorf("leaf path collapsed to its immediate parent:\n%s", res.Body)
	}
}

// TestDescribeKeepsNestedPositionalPath is the positional-composite counterpart:
// a custom spec with a nested positional composite (48.2.1) must keep "F48.2"
// and "48.2.1" rather than collapsing to "F2" and "2.1".
func TestDescribeKeepsNestedPositionalPath(t *testing.T) {
	t.Parallel()

	const spec = `{
  "name": "F48 nested positional",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "11": {"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},
    "48": {
      "type":"Composite","length":999,"description":"Private Data","prefix":"ASCII.LLL",
      "tag":{"sort":"StringsByInt"},
      "subfields": {
        "1": {"type":"String","length":2,"description":"A","enc":"ASCII","prefix":"ASCII.Fixed"},
        "2": {
          "type":"Composite","length":6,"description":"B","prefix":"ASCII.Fixed",
          "tag":{"sort":"StringsByInt"},
          "subfields": {"1": {"type":"String","length":6,"description":"Nested","enc":"ASCII","prefix":"ASCII.Fixed"}}
        }
      }
    }
  }
}`

	loaded := loadSpecFile(t, "48-nested.json", spec)
	doc := messageio.Document{
		MTI:    "0100",
		Fields: map[string]string{"11": "123456", "48.1": "AB", "48.2.1": "260604"},
	}
	packed, err := WriteMessage(doc, loaded)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	res, err := ViewMessage(packed.Raw, loaded, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false), false)
	if err != nil {
		t.Fatalf("ViewMessage describe: %v", err)
	}
	for _, want := range []string{"F48.2 B SUBFIELDS:", "48.2.1"} {
		if !strings.Contains(res.Body, want) {
			t.Errorf("describe output missing %q\n%s", want, res.Body)
		}
	}
}

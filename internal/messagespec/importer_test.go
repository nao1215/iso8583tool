package messagespec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/iso8583tool/internal/config"
)

// writeSpec writes a JSON spec to a temp file and returns its path.
func writeSpec(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return path
}

// loadSpec loads a JSON spec file and fails the test if it cannot be imported.
func loadSpec(t *testing.T, body string) *Spec {
	t.Helper()
	path := writeSpec(t, body)
	s, err := Load(filepath.Dir(path), config.Config{Spec: path})
	if err != nil {
		t.Fatalf("Load(%s): %v", path, err)
	}
	return s
}

func TestImportHexTopLevelField(t *testing.T) {
	t.Parallel()
	s := loadSpec(t, `{
  "name": "Hex field top",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "52": {"type":"Hex","length":8,"description":"PIN Data","enc":"Binary","prefix":"Binary.Fixed"}
  }
}`)
	if _, ok := s.MessageSpec.Fields[52]; !ok {
		t.Fatal("field 52 (Hex) should be defined")
	}
}

func TestImportHexSubfield(t *testing.T) {
	t.Parallel()
	s := loadSpec(t, `{
  "name": "TLV with Hex subfield",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "11": {"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},
    "55": {
      "type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL",
      "tag":{"enc":"BerTLVTag","sort":"StringsByHex","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},
      "subfields": {"9F02": {"type":"Hex","length":6,"description":"Amount","enc":"Binary","prefix":"BerTLV"}}
    }
  }
}`)
	if _, ok := s.MessageSpec.Fields[55]; !ok {
		t.Fatal("field 55 (Composite with Hex subfield) should be defined")
	}
}

func TestImportTrack1Field(t *testing.T) {
	t.Parallel()
	s := loadSpec(t, `{
  "name": "Track1 field",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "45": {"type":"Track1","length":76,"description":"Track 1","enc":"ASCII","prefix":"ASCII.LL"}
  }
}`)
	if _, ok := s.MessageSpec.Fields[45]; !ok {
		t.Fatal("field 45 (Track1) should be defined")
	}
}

func TestImportTrack3Field(t *testing.T) {
	t.Parallel()
	s := loadSpec(t, `{
  "name": "Track3 field",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "36": {"type":"Track3","length":104,"description":"Track 3","enc":"ASCII","prefix":"ASCII.LLL"}
  }
}`)
	if _, ok := s.MessageSpec.Fields[36]; !ok {
		t.Fatal("field 36 (Track3) should be defined")
	}
}

func TestImportIndexTagSubfield(t *testing.T) {
	t.Parallel()
	s := loadSpec(t, `{
  "name": "IndexTag composite",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "48": {
      "type":"Composite","length":999,"description":"IndexTag Composite","prefix":"ASCII.LLL",
      "tag":{"sort":"StringsByInt","length":2,"enc":"ASCII"},
      "subfields": {"1": {"type":"IndexTag","length":2,"description":"Tag index","enc":"ASCII","prefix":"ASCII.Fixed"}}
    }
  }
}`)
	if _, ok := s.MessageSpec.Fields[48]; !ok {
		t.Fatal("field 48 (Composite with IndexTag subfield) should be defined")
	}
}

func TestImportCompositeTagWithoutSort(t *testing.T) {
	t.Parallel()
	s := loadSpec(t, `{
  "name": "TLV no sort",
  "fields": {
    "0": {"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},
    "1": {"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},
    "11": {"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},
    "55": {
      "type":"Composite","length":999,"description":"ICC","prefix":"ASCII.LLL",
      "tag":{"enc":"BerTLVTag","skipUnknownTLVTags":true,"storeUnknownTLVTags":true},
      "subfields": {"9F02": {"type":"Binary","length":6,"description":"Amount","enc":"Binary","prefix":"BerTLV"}}
    }
  }
}`)
	if _, ok := s.MessageSpec.Fields[55]; !ok {
		t.Fatal("field 55 (Composite without tag sort) should be defined")
	}
}

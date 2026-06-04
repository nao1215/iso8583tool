package basei

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadCatalog(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "extensions.json")
	if err := SaveCatalog(path, DefaultExtensionCatalog()); err != nil {
		t.Fatalf("SaveCatalog returned error: %v", err)
	}

	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}

	field55, ok := catalog.Lookup(55)
	if !ok {
		t.Fatal("field 55 not found")
	}
	if field55.Strategy != StrategyTLV {
		t.Fatalf("field 55 strategy = %q, want %q", field55.Strategy, StrategyTLV)
	}
	if !field55.PreserveUnknownTLVTags {
		t.Fatal("field 55 should preserve unknown TLV tags")
	}
}

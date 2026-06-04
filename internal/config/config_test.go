package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
)

func TestDefaultUsesBuiltinCatalog(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if cfg.Spec != "" {
		t.Fatalf("Default().Spec = %q, want empty", cfg.Spec)
	}
	catalog := cfg.Catalog()
	if _, ok := catalog.Lookup(55); !ok {
		t.Fatal("default catalog should contain field 55")
	}
}

func TestLoadInlineCatalog(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	body := `{
  "spec": "spec87ascii",
  "extensions": [
    { "id": 55, "name": "ICC", "strategy": "tlv", "preserve_unknown_tlv_tags": true },
    { "id": 63, "name": "Private", "strategy": "opaque" }
  ]
}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Spec != "spec87ascii" {
		t.Fatalf("Spec = %q, want spec87ascii", cfg.Spec)
	}

	catalog := cfg.Catalog()
	if len(catalog.Fields) != 2 {
		t.Fatalf("catalog has %d fields, want 2", len(catalog.Fields))
	}
	field55, ok := catalog.Lookup(55)
	if !ok || field55.Strategy != basei.StrategyTLV || !field55.PreserveUnknownTLVTags {
		t.Fatalf("field 55 not loaded as expected: %#v", field55)
	}
}

func TestExplicitEmptyExtensionsDisablesCatalog(t *testing.T) {
	t.Parallel()

	// An explicit empty array must replace the built-in catalog with nothing,
	// matching the documented "the list replaces it" contract. Omitting the key
	// (tested separately) is what keeps the built-in default.
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, []byte(`{"extensions":[]}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	catalog := cfg.Catalog()
	if len(catalog.Fields) != 0 {
		t.Fatalf("explicit empty extensions should yield an empty catalog, got %d fields", len(catalog.Fields))
	}
	if _, ok := catalog.Lookup(55); ok {
		t.Fatal("explicit empty extensions must not fall back to the built-in catalog")
	}
}

func TestOmittedExtensionsUsesBuiltinCatalog(t *testing.T) {
	t.Parallel()

	// Omitting the key (as opposed to an explicit empty array) keeps the
	// built-in catalog so existing configs that only set a spec are unchanged.
	path := filepath.Join(t.TempDir(), "omitted.json")
	if err := os.WriteFile(path, []byte(`{"spec":"basei-starter"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if _, ok := cfg.Catalog().Lookup(55); !ok {
		t.Fatal("omitted extensions should fall back to the built-in catalog")
	}
}

func TestLoadRejectsInvalidStrategy(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad.json")
	body := `{ "extensions": [ { "id": 48, "name": "x", "strategy": "nope" } ] }`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load should reject an unknown strategy")
	}
}

func TestLoadRejectsDuplicateExtensionIDs(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "dup.json")
	body := `{ "extensions": [
    { "id": 55, "name": "ICC", "strategy": "tlv" },
    { "id": 55, "name": "ICC duplicate", "strategy": "opaque" }
  ] }`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load should reject duplicate extension ids")
	}
}

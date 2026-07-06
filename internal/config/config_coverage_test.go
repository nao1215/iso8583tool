package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestLoadMissingFile covers the os.ReadFile error branch.
func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("Load of a missing file should error")
	}
}

// TestLoadInvalidJSON covers the json.Unmarshal error branch.
func TestLoadInvalidJSON(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, "{not json")
	if _, err := Load(path); err == nil {
		t.Fatal("Load of invalid JSON should error")
	}
}

// TestLoadNonPositiveExtensionID covers the ext.ID <= 0 branch.
func TestLoadNonPositiveExtensionID(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `{"extensions":[{"id":0,"strategy":"tlv"}]}`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load with a non-positive extension id should error")
	}
}

// TestLoadDuplicateExtensionID covers the duplicate-id branch.
func TestLoadDuplicateExtensionID(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `{"extensions":[{"id":48,"strategy":"tlv"},{"id":48,"strategy":"opaque"}]}`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load with a duplicate extension id should error")
	}
}

// TestLoadInvalidStrategy covers the !Strategy.Valid() branch.
func TestLoadInvalidStrategy(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `{"extensions":[{"id":48,"strategy":"bogus"}]}`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load with an unsupported strategy should error")
	}
}

// TestLoadValid covers the success path and confirms the parsed catalog.
func TestLoadValid(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `{"spec":"spec87ascii","extensions":[{"id":48,"strategy":"tlv","name":"private"}]}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Spec != "spec87ascii" {
		t.Errorf("Spec = %q, want spec87ascii", cfg.Spec)
	}
	if len(cfg.Extensions) != 1 || cfg.Extensions[0].Strategy != basei.StrategyTLV {
		t.Errorf("unexpected extensions: %+v", cfg.Extensions)
	}
}

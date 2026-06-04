// Package config loads the optional, explicitly-passed settings file.
//
// A config is a single JSON file that can bundle a default message spec and an
// extension-field catalog. No file is required: built-in defaults apply when
// neither --config nor --spec is provided.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nao1215/iso8583tool/internal/basei"
)

// Config is the parsed --config file.
type Config struct {
	// Spec selects the default message spec for the config. Empty or
	// "basei-starter" uses the built-in BASE I starter; "spec87ascii" uses the
	// plain ISO 8583:1987 ASCII spec; "spec87bcd-starter" uses a raw binary /
	// packed-BCD starter; any other value is treated as a path to a
	// moov-io/iso8583 JSON spec, resolved relative to the config file. The CLI
	// --spec flag overrides this value when both are provided.
	Spec string `json:"spec,omitempty"`
	// Extensions is an inline extension-field catalog that replaces the built-in
	// one. A nil slice (the key omitted) keeps the built-in BASE I catalog; an
	// explicit empty array disables it entirely. The omitempty tag only affects
	// marshaling, so this omitted-vs-explicit-empty distinction survives a load.
	Extensions []basei.ExtensionField `json:"extensions,omitempty"`
}

// Default returns the zero config, which resolves to the built-in defaults.
func Default() Config { return Config{} }

// Load reads and validates a JSON config file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	seen := map[int]struct{}{}
	for _, ext := range cfg.Extensions {
		if ext.ID <= 0 {
			return Config{}, fmt.Errorf("extension id must be positive: %d", ext.ID)
		}
		if _, ok := seen[ext.ID]; ok {
			return Config{}, fmt.Errorf("duplicate extension id: %d", ext.ID)
		}
		seen[ext.ID] = struct{}{}
		if !ext.Strategy.Valid() {
			return Config{}, fmt.Errorf("unsupported extension strategy %q for field %d", ext.Strategy, ext.ID)
		}
	}
	return cfg, nil
}

// Catalog returns the configured extension catalog, or the built-in default
// when the extensions key was omitted. An explicit empty array replaces the
// built-in catalog with an empty one, so no extension fields are annotated.
func (c Config) Catalog() basei.ExtensionCatalog {
	if c.Extensions == nil {
		return basei.DefaultExtensionCatalog()
	}
	return basei.ExtensionCatalog{Fields: c.Extensions}
}

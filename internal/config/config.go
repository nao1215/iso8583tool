// Package config loads the optional, explicitly-passed settings file.
//
// A config is a single JSON file that bundles the message-spec selection and
// the extension-field catalog, so one --config path is enough. No file is
// required: with no config the built-in BASE I defaults are used.
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
	// Spec selects the message spec. Empty or "basei-starter" uses the
	// built-in BASE I starter; "spec87ascii" uses the plain ISO 8583:1987
	// ASCII spec; any other value is treated as a path to a moov-io/iso8583
	// JSON spec, resolved relative to the config file.
	Spec string `json:"spec,omitempty"`
	// Extensions is an inline extension-field catalog. When empty the
	// built-in BASE I catalog is used.
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
	for _, ext := range cfg.Extensions {
		if ext.ID <= 0 {
			return Config{}, fmt.Errorf("extension id must be positive: %d", ext.ID)
		}
		if !ext.Strategy.Valid() {
			return Config{}, fmt.Errorf("unsupported extension strategy %q for field %d", ext.Strategy, ext.ID)
		}
	}
	return cfg, nil
}

// Catalog returns the configured extension catalog, or the built-in default
// when none is specified.
func (c Config) Catalog() basei.ExtensionCatalog {
	if len(c.Extensions) == 0 {
		return basei.DefaultExtensionCatalog()
	}
	return basei.ExtensionCatalog{Fields: c.Extensions}
}

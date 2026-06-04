package basei

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const StarterPreset = "basei-starter"

type ExtensionStrategy string

const (
	StrategyOpaque     ExtensionStrategy = "opaque"
	StrategyTLV        ExtensionStrategy = "tlv"
	StrategyPositional ExtensionStrategy = "positional"
	StrategyBitmap     ExtensionStrategy = "bitmap"
)

type ExtensionField struct {
	ID                     int               `json:"id"`
	Name                   string            `json:"name"`
	Strategy               ExtensionStrategy `json:"strategy"`
	Description            string            `json:"description"`
	PreserveUnknownTLVTags bool              `json:"preserve_unknown_tlv_tags,omitempty"`
}

type ExtensionCatalog struct {
	Fields []ExtensionField `json:"fields"`
}

func DefaultExtensionCatalog() ExtensionCatalog {
	return ExtensionCatalog{
		Fields: []ExtensionField{
			{
				ID:          48,
				Name:        "Additional Data - Private",
				Strategy:    StrategyPositional,
				Description: "Use a positional overlay once the network-specific segment layout is fixed. Until then, keep field 48 editable as a single value.",
			},
			{
				ID:                     55,
				Name:                   "ICC System Related Data",
				Strategy:               StrategyTLV,
				Description:            "Treat EMV/ICC payloads as TLV and preserve unknown tags so round-trip edits do not drop issuer-specific data.",
				PreserveUnknownTLVTags: true,
			},
			{
				ID:          60,
				Name:        "Reserved National",
				Strategy:    StrategyPositional,
				Description: "Reserve for BASE I overlays that are stable enough to model as numbered subfields.",
			},
			{
				ID:          62,
				Name:        "Reserved Private",
				Strategy:    StrategyPositional,
				Description: "Prefer path-based editing (for example 62.1, 62.2) once the private layout is formally documented.",
			},
			{
				ID:          63,
				Name:        "Reserved Private",
				Strategy:    StrategyOpaque,
				Description: "Keep opaque until the private format is proven consistent across message classes.",
			},
			{
				ID:          126,
				Name:        "Reserved Private",
				Strategy:    StrategyOpaque,
				Description: "Late private extensions often drift by partner. Start opaque and promote only per-profile.",
			},
			{
				ID:          127,
				Name:        "Reserved Private",
				Strategy:    StrategyBitmap,
				Description: "Many switches use nested bitmaps or subelements here. Model this separately from flat field parsing.",
			},
		},
	}
}

func (s ExtensionStrategy) Valid() bool {
	switch s {
	case StrategyOpaque, StrategyTLV, StrategyPositional, StrategyBitmap:
		return true
	default:
		return false
	}
}

func (c ExtensionCatalog) Lookup(id int) (ExtensionField, bool) {
	for _, field := range c.Fields {
		if field.ID == id {
			return field, true
		}
	}
	return ExtensionField{}, false
}

func SaveCatalog(path string, catalog ExtensionCatalog) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	catalog.sort()
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Clean(path), data, 0o600)
}

func LoadCatalog(path string) (ExtensionCatalog, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return ExtensionCatalog{}, err
	}
	var catalog ExtensionCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return ExtensionCatalog{}, err
	}
	if len(catalog.Fields) == 0 {
		return ExtensionCatalog{}, errors.New("extension catalog must contain at least one field")
	}
	for _, field := range catalog.Fields {
		if field.ID <= 0 {
			return ExtensionCatalog{}, fmt.Errorf("extension field id must be positive: %d", field.ID)
		}
		if !field.Strategy.Valid() {
			return ExtensionCatalog{}, fmt.Errorf("unsupported extension strategy %q for field %d", field.Strategy, field.ID)
		}
	}
	catalog.sort()
	return catalog, nil
}

func (c *ExtensionCatalog) sort() {
	sort.Slice(c.Fields, func(i, j int) bool {
		return c.Fields[i].ID < c.Fields[j].ID
	})
}

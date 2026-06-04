package basei

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

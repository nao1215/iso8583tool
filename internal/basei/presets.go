package basei

import "github.com/moov-io/iso8583"

// Preset is a built-in message spec selectable via --spec NAME. The registry is
// the single source of truth for which presets exist, so the loader, the
// `specs` listing, and the `doctor` detector cannot drift apart.
type Preset struct {
	// Name is the value passed to --spec (and stored as the spec label).
	Name string
	// Title is the human-readable spec name.
	Title string
	// Encoding is a short description of how the wire bytes are encoded, which
	// is the property that distinguishes one preset from another.
	Encoding string
	// Summary is a one-line "when to use this" hint.
	Summary string
	// Default reports whether this preset is used when --spec is omitted.
	Default bool

	build func() *iso8583.MessageSpec
}

// Spec builds the moov-io message spec for the preset.
func (p Preset) Spec() *iso8583.MessageSpec { return p.build() }

// Presets returns the built-in presets in recommendation order: the default
// first, then the plainer and the more specialized layouts.
func Presets() []Preset {
	return []Preset{
		{
			Name:     StarterPreset,
			Title:    "BASE I Starter ASCII",
			Encoding: "ASCII fields, ASCII-hex bitmap, field 55 as EMV BER-TLV",
			Summary:  "Default. BASE I authorization/financial traffic with EMV ICC data in field 55.",
			Default:  true,
			build:    StarterMessageSpec,
		},
		{
			Name:     Spec87ASCIIPreset,
			Title:    "ISO 8583:1987 ASCII",
			Encoding: "ASCII fields, ASCII-hex bitmap",
			Summary:  "Plain 1987 ASCII layout without the BASE I field 55 EMV overlay.",
			build:    Spec87ASCIIWithSecondaryFields,
		},
		{
			Name:     Spec87BCDStarterPreset,
			Title:    "ISO 8583:1987 Packed BCD Starter",
			Encoding: "packed-BCD MTI and numeric fields, binary bitmap, binary length prefixes",
			Summary:  "Raw-binary captures; for example a *.bin message read with --encoding raw.",
			build:    Spec87BCDStarter,
		},
	}
}

// LookupPreset returns the preset with the given --spec name.
func LookupPreset(name string) (Preset, bool) {
	for _, p := range Presets() {
		if p.Name == name {
			return p, true
		}
	}
	return Preset{}, false
}

package service

import (
	"testing"

	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"
	moovsort "github.com/moov-io/iso8583/sort"

	"github.com/nao1215/iso8583tool/internal/basei"
)

func tlvComposite() *field.Composite {
	return field.NewComposite(&field.Spec{
		Length:      255,
		Description: "ICC",
		Pref:        prefix.ASCII.LLL,
		Tag:         &field.TagSpec{Enc: encoding.BerTLVTag, Sort: moovsort.StringsByHex},
		Subfields:   map[string]field.Field{"9F02": field.NewString(&field.Spec{Length: 6, Enc: encoding.Binary, Pref: prefix.BerTLV})},
	})
}

func positionalComposite() *field.Composite {
	return field.NewComposite(&field.Spec{
		Length:      8,
		Description: "Private",
		Pref:        prefix.ASCII.LLL,
		Tag:         &field.TagSpec{Sort: moovsort.StringsByInt}, // no Enc => positional
		Subfields:   map[string]field.Field{"1": field.NewString(&field.Spec{Length: 2, Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed})},
	})
}

func bitmapComposite() *field.Composite {
	return field.NewComposite(&field.Spec{
		Length:      255,
		Description: "Bitmap composite",
		Pref:        prefix.ASCII.LL,
		Bitmap:      field.NewBitmap(&field.Spec{Length: 8, Enc: encoding.BytesToASCIIHex, Pref: prefix.Hex.Fixed, DisableAutoExpand: true}),
		Subfields:   map[string]field.Field{"1": field.NewString(&field.Spec{Length: 2, Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed})},
	})
}

// TestDeriveStrategy guards that the extension section reports how the active
// spec actually models a field, not the catalog's BASE I assumption: a TLV
// composite is tlv, a positional one positional, a bitmap one bitmap, and any
// plain field opaque.
func TestDeriveStrategy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		f    field.Field
		want basei.ExtensionStrategy
	}{
		{"tlv composite", tlvComposite(), basei.StrategyTLV},
		{"positional composite", positionalComposite(), basei.StrategyPositional},
		{"bitmap composite", bitmapComposite(), basei.StrategyBitmap},
		{"plain string", field.NewString(&field.Spec{Length: 3, Enc: encoding.ASCII, Pref: prefix.ASCII.LLL}), basei.StrategyOpaque},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := deriveStrategy(tc.f, basei.StrategyTLV); got != tc.want {
				t.Fatalf("deriveStrategy(%s) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestActiveExtensionsReportsPerKindStrategy(t *testing.T) {
	t.Parallel()

	fields := map[int]field.Field{
		48:  positionalComposite(),
		55:  tlvComposite(),
		127: bitmapComposite(),
		63:  field.NewString(&field.Spec{Length: 3, Enc: encoding.ASCII, Pref: prefix.ASCII.LLL}),
	}
	catalog := basei.ExtensionCatalog{Fields: []basei.ExtensionField{
		{ID: 48, Name: "Private", Strategy: basei.StrategyTLV}, // catalog lies; spec wins
		{ID: 55, Name: "ICC", Strategy: basei.StrategyPositional},
		{ID: 127, Name: "Private use", Strategy: basei.StrategyTLV},
		{ID: 63, Name: "Reserved", Strategy: basei.StrategyBitmap},
	}}

	want := map[int]basei.ExtensionStrategy{
		48:  basei.StrategyPositional,
		55:  basei.StrategyTLV,
		127: basei.StrategyBitmap,
		63:  basei.StrategyOpaque,
	}
	for _, ext := range activeExtensions(fields, catalog) {
		if want[ext.ID] != ext.Strategy {
			t.Errorf("F%d strategy = %q, want %q", ext.ID, ext.Strategy, want[ext.ID])
		}
	}
}

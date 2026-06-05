package basei

import (
	"testing"

	"github.com/moov-io/iso8583"
)

func fieldSet(spec *iso8583.MessageSpec) map[int]bool {
	set := make(map[int]bool, len(spec.Fields))
	for id := range spec.Fields {
		set[id] = true
	}
	return set
}

func builtinFieldSets() map[string]map[int]bool {
	return map[string]map[int]bool{
		"basei-starter":     fieldSet(StarterMessageSpec()),
		"spec87ascii":       fieldSet(Spec87ASCIIWithSecondaryFields()),
		"spec87bcd-starter": fieldSet(Spec87BCDStarter()),
	}
}

// TestSecondaryBitmapFieldsDefined checks that the built-in presets define the
// standard high-numbered fields a real message can carry, so a document using
// them packs instead of failing with "field N is not defined in the spec".
func TestSecondaryBitmapFieldsDefined(t *testing.T) {
	t.Parallel()

	want := []int{95, 96, 100, 102, 103, 104, 123, 124, 125, 126, 127, 128}
	for name, set := range builtinFieldSets() {
		for _, id := range want {
			if !set[id] {
				t.Errorf("preset %s does not define field %d", name, id)
			}
		}
	}
}

// TestCatalogFieldsAreDefined guards the catalog/spec contract: every field the
// default extension catalog documents must exist in each built-in preset.
func TestCatalogFieldsAreDefined(t *testing.T) {
	t.Parallel()

	catalog := DefaultExtensionCatalog()
	for name, set := range builtinFieldSets() {
		for _, ext := range catalog.Fields {
			if !set[ext.ID] {
				t.Errorf("preset %s does not define catalog field %d (%s)", name, ext.ID, ext.Name)
			}
		}
	}
}

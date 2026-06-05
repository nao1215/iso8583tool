package basei

import (
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"
)

// TestIsBuiltinMessageSpec checks that the bundled presets are recognized and
// that a custom spec is not — even one that spoofs a bundled spec's name, since
// detection is by pointer identity, not by name. Misclassifying a custom spec as
// built-in would narrow cardholder masking, so this must not be spoofable.
func TestIsBuiltinMessageSpec(t *testing.T) {
	t.Parallel()

	for _, spec := range []*iso8583.MessageSpec{
		StarterMessageSpec(),
		Spec87ASCIIWithSecondaryFields(),
		Spec87BCDStarter(),
	} {
		if !IsBuiltinMessageSpec(spec) {
			t.Errorf("preset %q should be recognized as built-in", spec.Name)
		}
	}

	spoof := &iso8583.MessageSpec{
		Name: starterMessageSpecName, // same name as a bundled preset
		Fields: map[int]field.Field{
			0: field.NewString(&field.Spec{Length: 4, Pref: prefix.ASCII.Fixed}),
		},
	}
	if IsBuiltinMessageSpec(spoof) {
		t.Error("a custom spec reusing a bundled name must NOT be classified as built-in")
	}
	if IsBuiltinMessageSpec(nil) {
		t.Error("nil spec must not be built-in")
	}
}

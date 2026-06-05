package messagespec

import (
	"github.com/moov-io/iso8583/field"
	moovsort "github.com/moov-io/iso8583/sort"
	moovspecs "github.com/moov-io/iso8583/specs"
)

// init registers the moov-io field types and tag defaults that the upstream
// JSON importer does not wire up out of the box, so a spec exported by
// moov-io/iso8583 (or hand-written against it) loads with --spec PATH without
// surprising "no constructor for field type" or "unknown sort function" errors.
//
// moovspecs.FieldConstructor and moovspecs.SortExtToInt are exported map
// variables precisely so callers can extend the importer; we add only the
// entries upstream omits and never overwrite an existing one, so a future moov
// release that fills these in keeps its own definitions.
func init() {
	register("Hex", func(spec *field.Spec) field.Field { return field.NewHex(spec) })
	register("Track1", func(spec *field.Spec) field.Field { return field.NewTrack1(spec) })
	register("Track3", func(spec *field.Spec) field.Field { return field.NewTrack3(spec) })
	// moov-io has no field type backing the "IndexTag" name (IndexTag is internal
	// tag metadata, not a field), so an index-tagged composite subfield is read
	// as a positional string value, which is how such subfields are addressed.
	register("IndexTag", func(spec *field.Spec) field.Field { return field.NewString(spec) })

	// A composite tag may legitimately omit "sort"; default it to hex-tag order,
	// which suits the BER-TLV composites these specs overwhelmingly describe.
	if _, ok := moovspecs.SortExtToInt[""]; !ok {
		moovspecs.SortExtToInt[""] = moovsort.StringsByHex
	}
}

// register adds a field constructor under name unless one already exists.
func register(name string, ctor moovspecs.FieldConstructorFunc) {
	if _, ok := moovspecs.FieldConstructor[name]; !ok {
		moovspecs.FieldConstructor[name] = ctor
	}
}

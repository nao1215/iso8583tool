package basei

import (
	"maps"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"
	"github.com/moov-io/iso8583/sort"
	moovspecs "github.com/moov-io/iso8583/specs"
)

var starterMessageSpec = buildStarterMessageSpec()
var spec87ASCIIWithSecondaryFields = buildSpec87ASCIIWithSecondaryFields()
var spec87BCDStarter = buildSpec87BCDStarter()

func StarterMessageSpec() *iso8583.MessageSpec {
	return starterMessageSpec
}

func Spec87ASCIIWithSecondaryFields() *iso8583.MessageSpec {
	return spec87ASCIIWithSecondaryFields
}

func Spec87BCDStarter() *iso8583.MessageSpec {
	return spec87BCDStarter
}

func buildStarterMessageSpec() *iso8583.MessageSpec {
	fields := maps.Clone(spec87ASCIIWithSecondaryFields.Fields)
	fields[55] = field.NewComposite(field55Spec())

	return &iso8583.MessageSpec{
		Name:   "BASE I Starter ASCII",
		Fields: fields,
	}
}

func buildSpec87ASCIIWithSecondaryFields() *iso8583.MessageSpec {
	fields := maps.Clone(moovspecs.Spec87ASCII.Fields)
	fields[70] = field.NewString(&field.Spec{
		Length:      3,
		Description: "Network Management Information Code",
		Enc:         encoding.ASCII,
		Pref:        prefix.ASCII.Fixed,
	})

	return &iso8583.MessageSpec{
		Name:   "ISO 8583:1987 ASCII",
		Fields: fields,
	}
}

func buildSpec87BCDStarter() *iso8583.MessageSpec {
	fields := maps.Clone(spec87ASCIIWithSecondaryFields.Fields)

	fields[0] = cloneWithEncoding(fields[0], encoding.BCD, prefix.BCD.Fixed)
	fields[1] = field.NewBitmap(&field.Spec{
		Length:      8,
		Description: "Bitmap",
		Enc:         encoding.Binary,
		Pref:        prefix.Binary.Fixed,
	})
	fields[2] = cloneWithEncoding(fields[2], encoding.BCD, prefix.Binary.L)

	for _, id := range []int{
		3, 4, 5, 6, 7, 8, 9, 10,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		21, 22, 23, 24, 25, 26, 27, 28, 29, 30,
		31, 49, 50, 51, 53, 70,
	} {
		fields[id] = cloneWithEncoding(fields[id], encoding.BCD, prefix.BCD.Fixed)
	}

	// Variable-length fields keep their ASCII length prefix in moov's spec87ascii;
	// in a packed-BCD capture the length bytes are BCD, so a "06"-long field is a
	// single 0x06 byte rather than the ASCII pair 0x30 0x36. Re-prefix every
	// remaining variable-length field so its wire length matches the preset name.
	for id, f := range fields {
		if _, ok := f.(*field.Composite); ok {
			continue
		}
		s := f.Spec()
		if s == nil || s.Pref == nil {
			continue
		}
		if bcdPref := bcdLengthPrefix(s.Pref); bcdPref != nil {
			fields[id] = cloneWithEncoding(f, s.Enc, bcdPref)
		}
	}

	// PIN data (52) and the MAC (64) are raw secret bytes, not ASCII-hex text, so
	// a raw-binary capture carries them as fixed-length binary.
	fields[52] = rawBinaryField(fields[52], 8, "PIN Data")
	fields[64] = rawBinaryField(fields[64], 8, "Message Authentication Code (MAC)")

	// Field 55 is EMV BER-TLV (like the BASE I starter) so 55.<tag> packs and
	// round-trips; its outer length prefix is BCD to match the packed layout.
	emv := field55Spec()
	emv.Pref = prefix.BCD.LLL
	fields[55] = field.NewComposite(emv)

	return &iso8583.MessageSpec{
		Name:   "ISO 8583:1987 Packed BCD Starter",
		Fields: fields,
	}
}

// rawBinaryField returns a fixed-length binary field of the given byte length,
// used for raw secret fields (PIN, MAC) in the packed-BCD preset.
func rawBinaryField(src field.Field, length int, description string) field.Field {
	if src != nil && src.Spec() != nil && description == "" {
		description = src.Spec().Description
	}
	return field.NewBinary(&field.Spec{
		Length:      length,
		Description: description,
		Enc:         encoding.Binary,
		Pref:        prefix.Binary.Fixed,
	})
}

// bcdLengthPrefix maps an ASCII variable-length prefix to its BCD equivalent,
// returning nil for fixed-length or already-binary prefixes (which are left as
// they are).
func bcdLengthPrefix(p prefix.Prefixer) prefix.Prefixer {
	switch p.Inspect() {
	case "ASCII.L":
		return prefix.BCD.L
	case "ASCII.LL":
		return prefix.BCD.LL
	case "ASCII.LLL":
		return prefix.BCD.LLL
	case "ASCII.LLLL":
		return prefix.BCD.LLLL
	default:
		return nil
	}
}

func cloneWithEncoding(src field.Field, enc encoding.Encoder, pref prefix.Prefixer) field.Field {
	spec := src.Spec()
	if spec == nil {
		return src
	}
	cloned := &field.Spec{
		Length:      spec.Length,
		Description: spec.Description,
		Enc:         enc,
		Pref:        pref,
		Pad:         spec.Pad,
	}
	switch src.(type) {
	case *field.Numeric:
		return field.NewNumeric(cloned)
	case *field.String:
		return field.NewString(cloned)
	default:
		return src
	}
}

func field55Spec() *field.Spec {
	return &field.Spec{
		Length:      999,
		Description: "ICC System Related Data",
		Pref:        prefix.ASCII.LLL,
		Tag: &field.TagSpec{
			Sort:                sort.StringsByHex,
			Enc:                 encoding.BerTLVTag,
			SkipUnknownTLVTags:  true,
			StoreUnknownTLVTags: true,
		},
		Subfields: map[string]field.Field{
			"5F2A": newEMVHexField("Transaction Currency Code", 2),
			"71":   newEMVHexField("Issuer Script Template 1", 128),
			"72":   newEMVHexField("Issuer Script Template 2", 128),
			"82":   newEMVHexField("Application Interchange Profile", 2),
			"84":   newEMVHexField("Dedicated File Name", 16),
			"8A":   newEMVHexField("Authorisation Response Code", 2),
			"91":   newEMVHexField("Issuer Authentication Data", 16),
			"95":   newEMVHexField("Terminal Verification Results", 5),
			"9A":   newEMVHexField("Transaction Date", 3),
			"9C":   newEMVHexField("Transaction Type", 1),
			"9F02": newEMVHexField("Amount, Authorised (Numeric)", 6),
			"9F03": newEMVHexField("Amount, Other (Numeric)", 6),
			"9F09": newEMVHexField("Application Version Number", 2),
			"9F10": newEMVHexField("Issuer Application Data", 32),
			"9F1A": newEMVHexField("Terminal Country Code", 2),
			"9F1E": newEMVHexField("Interface Device Serial Number", 8),
			"9F26": newEMVHexField("Application Cryptogram", 8),
			"9F27": newEMVHexField("Cryptogram Information Data", 1),
			"9F33": newEMVHexField("Terminal Capabilities", 3),
			"9F34": newEMVHexField("Cardholder Verification Method Results", 3),
			"9F35": newEMVHexField("Terminal Type", 1),
			"9F36": newEMVHexField("Application Transaction Counter", 2),
			"9F37": newEMVHexField("Unpredictable Number", 4),
			"9F41": newEMVHexField("Transaction Sequence Counter", 4),
		},
	}
}

func newEMVHexField(description string, length int) field.Field {
	return field.NewHex(&field.Spec{
		Length:      length,
		Description: description,
		Enc:         encoding.Binary,
		Pref:        prefix.BerTLV,
	})
}

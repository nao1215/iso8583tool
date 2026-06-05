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

// The MessageSpec.Name of each bundled preset.
const (
	starterMessageSpecName     = "BASE I Starter ASCII"
	spec87ASCIIMessageSpecName = "ISO 8583:1987 ASCII"
	spec87BCDMessageSpecName   = "ISO 8583:1987 Packed BCD Starter"
)

// IsBuiltinMessageSpec reports whether spec is one of the bundled BASE I presets.
// Cardholder-data masking uses this to decide whether the BASE I positional
// field semantics apply: a custom --spec PATH assigns its own meaning to those
// field ids, so only content scanning (PAN/track-shaped values) and the known
// EMV cardholder tags apply there.
//
// The check is by pointer identity against the singleton preset specs, not by
// name, so a custom spec cannot be misclassified as built-in (and have its
// masking narrowed) by reusing a bundled spec's name.
func IsBuiltinMessageSpec(spec *iso8583.MessageSpec) bool {
	return spec != nil &&
		(spec == starterMessageSpec ||
			spec == spec87ASCIIWithSecondaryFields ||
			spec == spec87BCDStarter)
}

func buildStarterMessageSpec() *iso8583.MessageSpec {
	fields := maps.Clone(spec87ASCIIWithSecondaryFields.Fields)
	fields[55] = field.NewComposite(field55Spec())

	return &iso8583.MessageSpec{
		Name:   starterMessageSpecName,
		Fields: fields,
	}
}

func buildSpec87ASCIIWithSecondaryFields() *iso8583.MessageSpec {
	fields := maps.Clone(moovspecs.Spec87ASCII.Fields)

	// moov's Spec87ASCII stops at field 64 (plus 90). Add the standard ISO
	// 8583:1987 secondary-bitmap fields (65-128) so a message that uses them
	// packs instead of failing with "field N is not defined in the spec". Field
	// 65 is the extended-bitmap indicator, handled by the bitmap, so it is not a
	// data field here. Fields 90 is already defined by moov.
	for id, f := range secondaryBitmapFields() {
		fields[id] = f
	}

	return &iso8583.MessageSpec{
		Name:   spec87ASCIIMessageSpecName,
		Fields: fields,
	}
}

// NumericSecondaryFields lists the digits-only ISO 8583 fields whose value must
// be numeric: the primary numeric data elements and the numeric secondary-bitmap
// fields (message numbers, date-action, counts, amounts, original/replacement
// amounts, net settlement, and institution-identification codes). moov models
// these as String (a 42-digit field overflows a fixed-width integer), so strict
// validation checks the value is all digits rather than relying on the type.
var NumericSecondaryFields = []int{
	3, 4, 7, 11, 12, 13, 14, 15, 16, 17, 18, 19,
	66, 67, 68, 69, 70, 71, 72, 73,
	74, 75, 76, 77, 78, 79, 80, 81,
	82, 83, 84, 85, 86, 87, 88, 89, 90,
	93, 94, 95, 97, 99, 100,
}

// secondaryBitmapFields returns the standard ISO 8583:1987 fields 66-128
// (excluding 90, which moov already defines). Fixed numeric/alphanumeric fields
// use an ASCII fixed prefix; the variable trailing fields use ASCII LL/LLL; the
// two message-security/MAC fields are fixed binary like field 64.
func secondaryBitmapFields() map[int]field.Field {
	f := map[int]field.Field{
		66:  asciiFixed("Settlement Code", 1),
		67:  asciiFixed("Extended Payment Code", 2),
		68:  asciiFixed("Receiving Institution Country Code", 3),
		69:  asciiFixed("Settlement Institution Country Code", 3),
		70:  asciiFixed("Network Management Information Code", 3),
		71:  asciiFixed("Message Number", 4),
		72:  asciiFixed("Message Number Last", 4),
		73:  asciiFixed("Date, Action", 6),
		74:  asciiFixed("Credits, Number", 10),
		75:  asciiFixed("Credits, Reversal Number", 10),
		76:  asciiFixed("Debits, Number", 10),
		77:  asciiFixed("Debits, Reversal Number", 10),
		78:  asciiFixed("Transfer, Number", 10),
		79:  asciiFixed("Transfer, Reversal Number", 10),
		80:  asciiFixed("Inquiries, Number", 10),
		81:  asciiFixed("Authorizations, Number", 10),
		82:  asciiFixed("Credits, Processing Fee Amount", 12),
		83:  asciiFixed("Credits, Transaction Fee Amount", 12),
		84:  asciiFixed("Debits, Processing Fee Amount", 12),
		85:  asciiFixed("Debits, Transaction Fee Amount", 12),
		86:  asciiFixed("Credits, Amount", 16),
		87:  asciiFixed("Credits, Reversal Amount", 16),
		88:  asciiFixed("Debits, Amount", 16),
		89:  asciiFixed("Debits, Reversal Amount", 16),
		91:  asciiFixed("File Update Code", 1),
		92:  asciiFixed("File Security Code", 2),
		93:  asciiFixed("Response Indicator", 5),
		94:  asciiFixed("Service Indicator", 7),
		95:  asciiFixed("Replacement Amounts", 42),
		96:  binaryFixed("Message Security Code", 8),
		97:  asciiFixed("Amount, Net Settlement", 17),
		98:  asciiFixed("Payee", 25),
		99:  asciiVar("Settlement Institution Identification Code", 11, prefix.ASCII.LL),
		100: asciiVar("Receiving Institution Identification Code", 11, prefix.ASCII.LL),
		101: asciiVar("File Name", 17, prefix.ASCII.LL),
		102: asciiVar("Account Identification 1", 28, prefix.ASCII.LL),
		103: asciiVar("Account Identification 2", 28, prefix.ASCII.LL),
		104: asciiVar("Transaction Description", 100, prefix.ASCII.LLL),
		128: binaryFixed("Message Authentication Code (MAC)", 8),
	}
	// 105-127 are reserved (ISO / national / private) variable-length fields.
	for id := 105; id <= 127; id++ {
		f[id] = asciiVar(reservedFieldName(id), 999, prefix.ASCII.LLL)
	}
	return f
}

// reservedFieldName labels the reserved ranges of the 1987 layout.
func reservedFieldName(id int) string {
	switch {
	case id <= 111:
		return "Reserved for ISO Use"
	case id <= 119:
		return "Reserved for National Use"
	default:
		return "Reserved for Private Use"
	}
}

// asciiFixed builds a fixed-length ASCII field.
func asciiFixed(description string, length int) field.Field {
	return field.NewString(&field.Spec{
		Length:      length,
		Description: description,
		Enc:         encoding.ASCII,
		Pref:        prefix.ASCII.Fixed,
	})
}

// asciiVar builds a variable-length ASCII field with the given length prefix.
func asciiVar(description string, length int, pref prefix.Prefixer) field.Field {
	return field.NewString(&field.Spec{
		Length:      length,
		Description: description,
		Enc:         encoding.ASCII,
		Pref:        pref,
	})
}

// binaryFixed builds a fixed-length raw binary field, used for secret fields
// (such as the MAC in field 128) that a document carries via binary_fields.
func binaryFixed(description string, length int) field.Field {
	return field.NewBinary(&field.Spec{
		Length:      length,
		Description: description,
		Enc:         encoding.Binary,
		Pref:        prefix.Binary.Fixed,
	})
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
		// Secondary-bitmap numeric fields — settlement/payment/country codes,
		// message numbers, the date-action field, the count and amount fields, the
		// original/replacement amounts, and the net settlement amount — are packed
		// BCD in a packed-BCD capture, like the primary numeric fields.
		66, 67, 68, 69, 71, 72, 73,
		74, 75, 76, 77, 78, 79, 80, 81,
		82, 83, 84, 85, 86, 87, 88, 89, 90,
		93, 94, 95, 97,
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

	// Fields 99 and 100 are numeric institution-identification codes, so their
	// payload is BCD too, not only the length prefix the loop above set.
	for _, id := range []int{99, 100} {
		fields[id] = cloneWithEncoding(fields[id], encoding.BCD, prefix.BCD.LL)
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
		Name:   spec87BCDMessageSpecName,
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

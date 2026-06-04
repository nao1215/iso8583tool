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

	return &iso8583.MessageSpec{
		Name:   "ISO 8583:1987 Packed BCD Starter",
		Fields: fields,
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

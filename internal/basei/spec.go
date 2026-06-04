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

func StarterMessageSpec() *iso8583.MessageSpec {
	return starterMessageSpec
}

func buildStarterMessageSpec() *iso8583.MessageSpec {
	fields := maps.Clone(moovspecs.Spec87ASCII.Fields)
	fields[55] = field.NewComposite(field55Spec())

	return &iso8583.MessageSpec{
		Name:   "BASE I Starter ASCII",
		Fields: fields,
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

package basei

import (
	"maps"

	"github.com/nao1215/iso8583tool/internal/messageio"
)

type Sample struct {
	Name     string
	Summary  string
	Document messageio.Document
}

func StarterSamples() []Sample {
	return []Sample{
		{
			Name:     "0100-auth-request",
			Summary:  "EMV authorization request with BASE I style private fields 48 and 62",
			Document: AuthRequest(),
		},
		{
			Name:     "0110-auth-response",
			Summary:  "EMV authorization response with issuer data in field 55 and opaque field 63",
			Document: AuthResponse(),
		},
		{
			Name:     "0200-financial-request",
			Summary:  "Card-present purchase request with EMV data and BASE I style private fields",
			Document: FinancialRequest(),
		},
		{
			Name:     "0210-financial-response",
			Summary:  "Approved financial response with issuer response code and settlement note",
			Document: FinancialResponse(),
		},
		{
			Name:     "0420-reversal-advice",
			Summary:  "Reversal advice carrying original data elements for timeout recovery",
			Document: ReversalAdvice(),
		},
		{
			Name:     "0430-reversal-response",
			Summary:  "Reversal advice response confirming the original purchase was reversed",
			Document: ReversalResponse(),
		},
		{
			Name:     "0800-network-echo",
			Summary:  "Network management echo test request using NMIC 301",
			Document: NetworkEchoRequest(),
		},
		{
			Name:     "0810-network-echo-response",
			Summary:  "Network management echo test response confirming host reachability",
			Document: NetworkEchoResponse(),
		},
	}
}

func LookupSample(name string) (Sample, bool) {
	for _, sample := range StarterSamples() {
		if sample.Name == name {
			return sample, true
		}
	}
	return Sample{}, false
}

func AuthRequest() messageio.Document {
	return messageio.Document{
		MTI: "0100",
		Fields: map[string]string{
			"2":  "4111111111111111",
			"3":  "000000",
			"4":  "000000005000",
			"7":  "0604123456",
			"11": "123456",
			"12": "123456",
			"13": "0604",
			"14": "2912",
			"18": "5999",
			"22": "051",
			"23": "001",
			"24": "200",
			"25": "00",
			"35": "4111111111111111D29122011234567890",
			"37": "REF123456789",
			"41": "TERMID01",
			"42": "MERCHANT0000001",
			"48": "LOYALTY=OFF|INSTALLMENT=00",
			"49": "392",
			"62": "ORDERID=000123|CHANNEL=ECOM",
		},
		BinaryFields: map[string]string{
			"55.5F2A": "0392",
			"55.82":   "3900",
			"55.84":   "A0000000031010",
			"55.95":   "8000008000",
			"55.9A":   "260604",
			"55.9C":   "00",
			"55.9F02": "000000005000",
			"55.9F03": "000000000000",
			"55.9F09": "008C",
			"55.9F10": "06011203A0B0C0",
			"55.9F1A": "0392",
			"55.9F1E": "5445535431323334",
			"55.9F26": "1122334455667788",
			"55.9F27": "80",
			"55.9F33": "E0F8C8",
			"55.9F34": "1F0302",
			"55.9F35": "22",
			"55.9F36": "0034",
			"55.9F37": "11223344",
			"55.9F41": "00000001",
		},
	}
}

func AuthResponse() messageio.Document {
	return messageio.Document{
		MTI: "0110",
		Fields: map[string]string{
			"2":  "4111111111111111",
			"3":  "000000",
			"4":  "000000005000",
			"7":  "0604123456",
			"11": "123456",
			"12": "123457",
			"13": "0604",
			"37": "REF123456789",
			"38": "A12345",
			"39": "00",
			"41": "TERMID01",
			"42": "MERCHANT0000001",
			"48": "APPROVED=Y|BALANCE=NA",
			"49": "392",
			"63": "NETWORKTRACE=OK",
		},
		BinaryFields: map[string]string{
			"55.71": "112233445566",
			"55.72": "AABBCC",
			"55.8A": "3030",
			"55.91": "A1B2C3D4E5F60708",
		},
	}
}

func FinancialRequest() messageio.Document {
	doc := cloneDocument(AuthRequest())
	doc.MTI = "0200"
	doc.Fields["4"] = "000000012345"
	doc.Fields["7"] = "0604130105"
	doc.Fields["11"] = "223344"
	doc.Fields["12"] = "130105"
	doc.Fields["22"] = "071"
	doc.Fields["35"] = "4111111111111111D29122011234567890"
	doc.Fields["37"] = "FIN200000001"
	doc.Fields["41"] = "TERMID02"
	doc.Fields["42"] = "MERCHANT0000002"
	doc.Fields["48"] = "INVOICE=900001|TIP=000000000500"
	doc.Fields["49"] = "840"
	doc.Fields["62"] = "ORDERID=000900|CHANNEL=POS"
	doc.BinaryFields["55.5F2A"] = "0840"
	doc.BinaryFields["55.9F02"] = "000000012345"
	doc.BinaryFields["55.9F1A"] = "0840"
	doc.BinaryFields["55.9F36"] = "0042"
	doc.BinaryFields["55.9F41"] = "00000002"
	return doc
}

func FinancialResponse() messageio.Document {
	doc := cloneDocument(AuthResponse())
	doc.MTI = "0210"
	doc.Fields["4"] = "000000012345"
	doc.Fields["7"] = "0604130105"
	doc.Fields["11"] = "223344"
	doc.Fields["12"] = "130106"
	doc.Fields["37"] = "FIN200000001"
	doc.Fields["38"] = "B65432"
	doc.Fields["41"] = "TERMID02"
	doc.Fields["42"] = "MERCHANT0000002"
	doc.Fields["48"] = "APPROVED=Y|BATCH=PENDING"
	doc.Fields["49"] = "840"
	doc.Fields["63"] = "SETTLEMENT=QUEUED"
	doc.BinaryFields["55.8A"] = "3030"
	doc.BinaryFields["55.91"] = "CAFEBABE01020304"
	return doc
}

func ReversalAdvice() messageio.Document {
	doc := cloneDocument(FinancialRequest())
	doc.MTI = "0420"
	doc.Fields["7"] = "0604151515"
	doc.Fields["11"] = "223355"
	doc.Fields["12"] = "151515"
	doc.Fields["37"] = "REV223355001"
	doc.Fields["48"] = "REVERSAL=TIMEOUT|REASON=NO_ISSUER_REPLY"
	doc.Fields["90"] = "020022334406041301050000000000000000000000"
	doc.BinaryFields["55.9F36"] = "0043"
	doc.BinaryFields["55.9F41"] = "00000003"
	return doc
}

func ReversalResponse() messageio.Document {
	doc := cloneDocument(ReversalAdvice())
	doc.MTI = "0430"
	doc.Fields["12"] = "151516"
	doc.Fields["38"] = "R76543"
	doc.Fields["39"] = "00"
	doc.Fields["48"] = "REVERSAL=ACCEPTED|FOLLOWUP=NONE"
	doc.Fields["63"] = "REVERSAL=BOOKED"
	doc.BinaryFields = map[string]string{
		"55.8A": "3030",
		"55.91": "0A0B0C0D0E0F1011",
	}
	return doc
}

func NetworkEchoRequest() messageio.Document {
	return messageio.Document{
		MTI: "0800",
		Fields: map[string]string{
			"7":  "0604161616",
			"11": "654321",
			"12": "161616",
			"13": "0604",
			"24": "001",
			"41": "TERMNET1",
			"48": "HEARTBEAT=BASEI",
			"70": "301",
		},
	}
}

func NetworkEchoResponse() messageio.Document {
	doc := cloneDocument(NetworkEchoRequest())
	doc.MTI = "0810"
	doc.Fields["12"] = "161617"
	doc.Fields["39"] = "00"
	doc.Fields["63"] = "ECHO=OK"
	return doc
}

func cloneDocument(doc messageio.Document) messageio.Document {
	return messageio.Document{
		MTI:          doc.MTI,
		Fields:       maps.Clone(doc.Fields),
		BinaryFields: maps.Clone(doc.BinaryFields),
	}
}

package basei

import "github.com/nao1215/iso8583tool/internal/messageio"

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
			"2":  "4761739001010010",
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
			"35": "4761739001010010D29122011234567890",
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
			"2":  "4761739001010010",
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

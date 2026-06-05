package annotate

import "testing"

func TestMTI(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"0100": "Authorization Request from Acquirer (ISO8583:1987)",
		"0110": "Authorization Response from Acquirer (ISO8583:1987)",
		"0200": "Financial Request from Acquirer (ISO8583:1987)",
		"0210": "Financial Response from Acquirer (ISO8583:1987)",
		"0420": "Reversal Advice from Acquirer (ISO8583:1987)",
		"0800": "Network management Request from Acquirer (ISO8583:1987)",
		"0810": "Network management Response from Acquirer (ISO8583:1987)",
	}
	for mti, want := range cases {
		if got := MTI(mti); got != want {
			t.Errorf("MTI(%q) = %q, want %q", mti, got, want)
		}
	}
	for _, bad := range []string{"", "01", "abcd", "01000"} {
		if got := MTI(bad); got != "" {
			t.Errorf("MTI(%q) = %q, want empty", bad, got)
		}
	}
}

func TestFieldMeaning(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path, value, want string
	}{
		{"3", "0", "Purchase / goods & services"}, // moov strips leading zeros
		{"3", "201234", "Refund / return"},        // refund processing code
		{"39", "00", "Approved"},                  // response code
		{"39", "54", "Expired card"},              // response code
		{"49", "392", "JPY (Japanese yen)"},       // currency, 3 digits
		{"55.5F2A", "0392", "JPY (Japanese yen)"}, // currency, padded hex form
		{"55.9F1A", "0392", "Japan"},              // country code
		{"22", "051", "Integrated circuit card (ICC); PIN entry capable"},
		{"25", "00", "Normal presentment"},             // POS condition
		{"70", "301", "Echo test"},                     // network management code
		{"55.9C", "00", "Purchase / goods & services"}, // EMV transaction type
		{"55.9F27", "80", "ARQC (online authorization requested)"},
		{"55.8A", "3030", "Approved"},                // ARC = ASCII "00"
		{"55.70.8A", "3030", "Approved"},             // ARC nested inside a constructed template
		{"55.70.9A", "260605", "2026-06-05"},         // EMV date nested inside a template
		{"55.77.5F2A", "0392", "JPY (Japanese yen)"}, // currency, two templates deep
	}
	for _, c := range cases {
		got, ok := FieldMeaning(c.path, c.value)
		if !ok || got != c.want {
			t.Errorf("FieldMeaning(%q, %q) = (%q, %v), want (%q, true)", c.path, c.value, got, ok, c.want)
		}
	}

	if _, ok := FieldMeaning("39", "ZZ"); ok {
		t.Error("FieldMeaning unknown response code should not match")
	}
	if _, ok := FieldMeaning("70", "999"); ok {
		t.Error("FieldMeaning unknown network code should not match")
	}
	if _, ok := FieldMeaning("11", "123456"); ok {
		t.Error("FieldMeaning for a non-coded field should not match")
	}
}

func TestNormalizeFieldPath(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"39", "39"},
		{"55.8A", "55.8A"},
		{"55.70.8A", "55.8A"},
		{"55.70.77.9A", "55.9A"},
	}
	for _, c := range cases {
		if got := normalizeFieldPath(c.in); got != c.want {
			t.Errorf("normalizeFieldPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFieldMeaningVariants(t *testing.T) {
	t.Parallel()
	cases := []struct{ path, value, want string }{
		{"22", "071", "Contactless ICC; PIN entry capable"},
		{"22", "022", "Magnetic stripe; no PIN entry"},
		{"55.9F27", "40", "TC (approved offline)"},
		{"55.9F27", "00", "AAC (declined offline)"},
		{"3", "201234", "Refund / return"},
	}
	for _, c := range cases {
		got, ok := FieldMeaning(c.path, c.value)
		if !ok || got != c.want {
			t.Errorf("FieldMeaning(%q,%q) = (%q,%v), want (%q,true)", c.path, c.value, got, ok, c.want)
		}
	}
	if _, ok := FieldMeaning("55.9F27", "zz"); ok {
		t.Error("invalid cryptogram hex should not match")
	}
}

func TestFieldMeaningDates(t *testing.T) {
	t.Parallel()
	cases := []struct{ path, value, want string }{
		{"7", "0604123456", "06-04 12:34:56"},
		{"12", "123456", "12:34:56"},
		{"13", "0604", "06-04"},
		{"14", "2912", "2029-12"},
		{"55.9A", "260604", "2026-06-04"},
	}
	for _, c := range cases {
		got, ok := FieldMeaning(c.path, c.value)
		if !ok || got != c.want {
			t.Errorf("FieldMeaning(%q, %q) = (%q, %v), want (%q, true)", c.path, c.value, got, ok, c.want)
		}
	}
}

func TestFormatAmount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		amount, currency, want string
		ok                     bool
	}{
		{"000000005000", "392", "JPY 5000", true},   // JPY: no minor unit
		{"000000005000", "840", "USD 50.00", true},  // USD: 2 decimals
		{"000000012345", "978", "EUR 123.45", true}, // EUR: 2 decimals
		{"000000005000", "999", "", false},          // unknown currency
	}
	for _, c := range cases {
		got, ok := FormatAmount(c.amount, c.currency)
		if ok != c.ok || got != c.want {
			t.Errorf("FormatAmount(%q, %q) = (%q, %v), want (%q, %v)", c.amount, c.currency, got, ok, c.want, c.ok)
		}
	}
}

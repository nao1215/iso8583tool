package annotate

import "testing"

func TestMTI(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"0100": "Authorization Request from Acquirer (ISO8583:1987)",
		"0110": "Authorization Request response from Acquirer (ISO8583:1987)",
		"0800": "Network management Request from Acquirer (ISO8583:1987)",
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
		{"55.9C", "00", "Purchase / goods & services"}, // EMV transaction type
		{"55.9F27", "80", "ARQC (online authorization requested)"},
		{"55.8A", "3030", "Approved"}, // ARC = ASCII "00"
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
	if _, ok := FieldMeaning("11", "123456"); ok {
		t.Error("FieldMeaning for a non-coded field should not match")
	}
}

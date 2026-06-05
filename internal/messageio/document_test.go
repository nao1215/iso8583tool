package messageio

import (
	"strings"
	"testing"
)

func TestParseDocument(t *testing.T) {
	t.Parallel()

	if _, err := ParseDocument([]byte(`{"mti":"0100","fields":{"2":"x"}}`)); err != nil {
		t.Fatalf("valid document: %v", err)
	}
	if _, err := ParseDocument([]byte(`not json`)); err == nil {
		t.Fatal("expected a JSON parse error")
	}
	if _, err := ParseDocument([]byte(`{"fields":{}}`)); err == nil {
		t.Fatal("expected a missing-mti error")
	}
}

func TestParseDocumentRejectsDuplicatePath(t *testing.T) {
	t.Parallel()

	// 55.8A appears in both fields and binary_fields: ambiguous, must fail.
	_, err := ParseDocument([]byte(`{"mti":"0100","fields":{"55.8A":"00"},"binary_fields":{"55.8A":"3035"}}`))
	if err == nil {
		t.Fatal("a path present in both fields and binary_fields must be rejected")
	}
	if !strings.Contains(err.Error(), "55.8A") {
		t.Fatalf("error should name the conflicting path, got %v", err)
	}
}

func TestParseDocumentRejectsParentChildConflict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  string
		want []string
	}{
		{"tlv root and tag", `{"mti":"0100","binary_fields":{"55":"9F0206000000005000","55.9F02":"000000009999"}}`, []string{"55", "55.9F02"}},
		{"nested bitmap", `{"mti":"0100","fields":{"127":"x"},"binary_fields":{"127.25.1":"00"}}`, []string{"127", "127.25.1"}},
		{"positional", `{"mti":"0100","fields":{"48":"x","48.1":"y"}}`, []string{"48", "48.1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseDocument([]byte(tc.doc))
			if err == nil {
				t.Fatalf("parent/child path conflict must be rejected: %s", tc.doc)
			}
			for _, p := range tc.want {
				if !strings.Contains(err.Error(), p) {
					t.Fatalf("error %q should name path %q", err, p)
				}
			}
		})
	}
}

func TestLooksLikeJSON(t *testing.T) {
	t.Parallel()

	if !LooksLikeJSON([]byte("  \n  {\"mti\":\"0100\"}")) {
		t.Fatal("leading whitespace before { should be JSON")
	}
	if LooksLikeJSON([]byte("303130300100")) {
		t.Fatal("hex should not be JSON")
	}
	if LooksLikeJSON([]byte("   \n")) {
		t.Fatal("blank should not be JSON")
	}
}

func TestLooksLikeJSONWithBOM(t *testing.T) {
	t.Parallel()

	// A UTF-8 BOM (EF BB BF) before the object must not hide the JSON. bug 12
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"mti":"0100"}`)...)
	if !LooksLikeJSON(bom) {
		t.Fatal("a BOM-prefixed object should be detected as JSON")
	}
}

func TestParseDocumentWithBOM(t *testing.T) {
	t.Parallel()

	// json.Unmarshal rejects a leading BOM, so ParseDocument must strip it. bug 13
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"mti":"0100","fields":{"11":"123456"}}`)...)
	doc, err := ParseDocument(bom)
	if err != nil {
		t.Fatalf("BOM-prefixed document: %v", err)
	}
	if doc.MTI != "0100" {
		t.Fatalf("doc.MTI = %q, want 0100", doc.MTI)
	}
}

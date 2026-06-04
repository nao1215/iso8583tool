package messageio

import "testing"

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

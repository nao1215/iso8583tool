package messageio

import (
	"strings"
	"testing"
)

func TestCanonicalPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
		want string
	}{
		{"plain id", "2", "2"},
		{"leading zero collapses", "02", "2"},
		{"longer leading zero", "099", "99"},
		{"tlv tag upper-cases", "55.9f02", "55.9F02"},
		{"nested tlv tag upper-cases", "55.70.8a", "55.70.8A"},
		{"positional stays", "48.2.1", "48.2.1"},
		{"max field id", "128", "128"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalPath(tc.path)
			if err != nil {
				t.Fatalf("canonicalPath(%q) unexpected error: %v", tc.path, err)
			}
			if got != tc.want {
				t.Fatalf("canonicalPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestCanonicalPathRejects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		path    string
		wantSub string
	}{
		{"mti reserved", "0", "reserved for mti"},
		{"bitmap reserved", "1", "bitmap"},
		{"non numeric id", "A", "invalid field id"},
		{"non numeric nested id", "A.1", "invalid field id"},
		{"negative id", "-2", "invalid field id"},
		{"above range", "129", "invalid field id"},
		{"far above range", "999", "invalid field id"},
		{"leading whitespace", " 2", "whitespace"},
		{"trailing whitespace", "2 ", "whitespace"},
		{"leading whitespace nested", " 55.9F02", "whitespace"},
		{"trailing whitespace nested", "55.9F02 ", "whitespace"},
		{"double dot", "55..9F02", "empty segment"},
		{"trailing dot", "55.", "empty segment"},
		{"leading dot", ".55", "empty segment"},
		{"deep trailing dot", "55.70.", "empty segment"},
		{"empty", "", "empty path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := canonicalPath(tc.path)
			if err == nil {
				t.Fatalf("canonicalPath(%q) should be rejected", tc.path)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("canonicalPath(%q) error = %q, want substring %q", tc.path, err, tc.wantSub)
			}
		})
	}
}

func TestParseDocumentRejectsReservedAndMalformedPaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{"field 0 overwrites mti", `{"mti":"0100","fields":{"0":"9999"}}`, "reserved for mti"},
		{"field 1 is bitmap", `{"mti":"0100","fields":{"1":"1234"}}`, "bitmap"},
		{"binary field 0", `{"mti":"0100","binary_fields":{"0":"31323334"}}`, "reserved for mti"},
		{"binary field 1", `{"mti":"0100","binary_fields":{"1":"31323334"}}`, "bitmap"},
		{"out of range", `{"mti":"0100","fields":{"129":"x"}}`, "invalid field id"},
		{"non numeric", `{"mti":"0100","fields":{"A.1":"x"}}`, "invalid field id"},
		{"malformed double dot", `{"mti":"0100","binary_fields":{"55..9F02":"00"}}`, "empty segment"},
		{"malformed trailing dot", `{"mti":"0100","binary_fields":{"55.":"00"}}`, "empty segment"},
		{"leading whitespace", `{"mti":"0100","fields":{" 2":"4111111111111111"}}`, "whitespace"},
		{"trailing whitespace", `{"mti":"0100","fields":{"2 ":"4111111111111111"}}`, "whitespace"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseDocument([]byte(tc.doc))
			if err == nil {
				t.Fatalf("document %s must be rejected", tc.doc)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q should contain %q", err, tc.wantSub)
			}
		})
	}
}

func TestParseDocumentRejectsCanonicalDuplicates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  string
	}{
		{"leading zero alias", `{"mti":"0100","fields":{"02":"4111111111111111","2":"4222222222222222"}}`},
		{"longer leading zero alias", `{"mti":"0100","fields":{"99":"a","099":"b"}}`},
		{"tlv case alias", `{"mti":"0100","binary_fields":{"55.9f02":"000000001000","55.9F02":"000000005000"}}`},
		{"nested tlv case alias", `{"mti":"0100","binary_fields":{"55.70.8A":"3030","55.70.8a":"3131"}}`},
		{"alias across maps", `{"mti":"0100","fields":{"2":"a"},"binary_fields":{"02":"4111"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseDocument([]byte(tc.doc)); err == nil {
				t.Fatalf("duplicate-alias document %s must be rejected", tc.doc)
			}
		})
	}
}

func TestPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw      string
		topLevel string
		isTop    bool
		leaf     string
		hasLeaf  bool
		segs     int
	}{
		{"55", "55", true, "", false, 1},
		{"55.9F02", "55", false, "9F02", true, 2},
		{"55.70.9F02", "55", false, "9F02", true, 3},
		{"48.2.1", "48", false, "1", true, 3},
		{"mti", "mti", true, "", false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			p := NewPath(tc.raw)
			if p.String() != tc.raw {
				t.Errorf("String() = %q, want %q", p.String(), tc.raw)
			}
			if p.TopLevelID() != tc.topLevel {
				t.Errorf("TopLevelID() = %q, want %q", p.TopLevelID(), tc.topLevel)
			}
			if p.IsTopLevel() != tc.isTop {
				t.Errorf("IsTopLevel() = %v, want %v", p.IsTopLevel(), tc.isTop)
			}
			leaf, ok := p.Leaf()
			if ok != tc.hasLeaf || leaf != tc.leaf {
				t.Errorf("Leaf() = (%q, %v), want (%q, %v)", leaf, ok, tc.leaf, tc.hasLeaf)
			}
			if len(p.Segments()) != tc.segs {
				t.Errorf("len(Segments()) = %d, want %d", len(p.Segments()), tc.segs)
			}
		})
	}
}

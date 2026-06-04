package messageio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSourceRejectsOversizedInline(t *testing.T) {
	t.Parallel()

	huge := strings.Repeat("0", MaxInputSize+1)
	if _, err := ReadSource("", huge, nil); err == nil {
		t.Fatal("expected oversized inline input to be rejected")
	}
}

func TestReadSourceRejectsOversizedStdin(t *testing.T) {
	t.Parallel()

	// A reader that always has more data than the cap allows.
	r := strings.NewReader(strings.Repeat("A", MaxInputSize+10))
	if _, err := ReadSource("-", "", r); err == nil {
		t.Fatal("expected oversized stdin to be rejected")
	}
}

func TestReadSourceRejectsOversizedFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "big.hex")
	if err := os.WriteFile(path, make([]byte, MaxInputSize+1), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if _, err := ReadSource(path, "", nil); err == nil {
		t.Fatal("expected oversized file to be rejected")
	}
}

func TestReadSourceAcceptsInputAtLimit(t *testing.T) {
	t.Parallel()

	// Exactly at the cap (and not over it) must still be read in full.
	path := filepath.Join(t.TempDir(), "ok.bin")
	want := make([]byte, MaxInputSize)
	for i := range want {
		want[i] = byte('a' + i%26)
	}
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	got, err := ReadSource(path, "", nil)
	if err != nil {
		t.Fatalf("ReadSource at the limit returned error: %v", err)
	}
	if len(got) != MaxInputSize {
		t.Fatalf("read %d bytes, want %d", len(got), MaxInputSize)
	}
}

func TestLooksLikeHex(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []byte
		want bool
	}{
		{"plain hex", []byte("3031323300"), true},
		{"upper hex", []byte("DEADBEEF"), true},
		{"hex with whitespace", []byte("30 31\n32 33"), true},
		{"odd length", []byte("303"), false},
		{"empty", []byte(""), false},
		{"whitespace only", []byte("  \n"), false},
		{"raw binary with control byte", []byte{0x01, 0x00, 0x70, 0x04}, false},
		{"non-hex letters", []byte("hello!"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := LooksLikeHex(tc.in); got != tc.want {
				t.Errorf("LooksLikeHex(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

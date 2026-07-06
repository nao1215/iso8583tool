package messagespec

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/config"
)

// TestLoadErrorMessages locks in the user-facing wording that distinguishes a
// mistyped preset from a bad file path and reports a missing file plainly.
func TestLoadErrorMessages(t *testing.T) {
	t.Parallel()

	cases := []struct {
		spec string
		want string
	}{
		// A bare word (no separator, no extension) reads as a preset typo.
		{"basei-startr", "run \"iso8583tool specs\""},
		{"spec87asci", "not a built-in preset"},
		// A path with a non-JSON extension is a file-format complaint.
		{"layout.yaml", "only JSON specs are supported"},
		{"dir/layout.txt", "only JSON specs are supported"},
	}
	for _, tc := range cases {
		_, err := Load(t.TempDir(), config.Config{Spec: tc.spec})
		if err == nil {
			t.Errorf("Load(%q) should fail", tc.spec)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Load(%q) error = %q, want substring %q", tc.spec, err.Error(), tc.want)
		}
	}

	// A .json path that does not exist reports a plain not-found, not os.Open noise.
	if _, err := Load(t.TempDir(), config.Config{Spec: "missing.json"}); err == nil ||
		!strings.Contains(err.Error(), "spec file not found") {
		t.Errorf("missing .json should report 'spec file not found', got %v", err)
	}
}

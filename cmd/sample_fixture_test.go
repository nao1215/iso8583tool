package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
)

// readFixture reads a fixture and normalizes CRLF to LF so a Windows checkout
// (autocrlf) does not cause a spurious mismatch against the LF output.
func readFixture(t *testing.T, path string) string {
	t.Helper()
	//nolint:gosec // fixture path is built from a built-in sample name
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func TestBundledSamplesMatchFixtures(t *testing.T) {
	t.Parallel()

	fixtureDir := filepath.Join("..", "examples", "basei")
	for _, sample := range basei.StarterSamples() {
		t.Run(sample.Name, func(t *testing.T) {
			t.Parallel()

			code, jsonOut, stderr := runApp("", "sample", sample.Name)
			if code != 0 {
				t.Fatalf("sample json failed: code=%d stderr=%q", code, stderr)
			}
			if jsonOut != readFixture(t, filepath.Join(fixtureDir, sample.Name+".json")) {
				t.Fatalf("json fixture drift for %s", sample.Name)
			}

			code, hexOut, stderr := runApp("", "sample", sample.Name, "--format", "hex")
			if code != 0 {
				t.Fatalf("sample hex failed: code=%d stderr=%q", code, stderr)
			}
			if hexOut != readFixture(t, filepath.Join(fixtureDir, sample.Name+".hex")) {
				t.Fatalf("hex fixture drift for %s", sample.Name)
			}
		})
	}
}

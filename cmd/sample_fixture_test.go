package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
)

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
			//nolint:gosec // fixture path is built from a built-in sample name
			wantJSON, err := os.ReadFile(filepath.Join(fixtureDir, sample.Name+".json"))
			if err != nil {
				t.Fatalf("read json fixture: %v", err)
			}
			if jsonOut != string(wantJSON) {
				t.Fatalf("json fixture drift for %s", sample.Name)
			}

			code, hexOut, stderr := runApp("", "sample", sample.Name, "--format", "hex")
			if code != 0 {
				t.Fatalf("sample hex failed: code=%d stderr=%q", code, stderr)
			}
			//nolint:gosec // fixture path is built from a built-in sample name
			wantHex, err := os.ReadFile(filepath.Join(fixtureDir, sample.Name+".hex"))
			if err != nil {
				t.Fatalf("read hex fixture: %v", err)
			}
			if hexOut != string(wantHex) {
				t.Fatalf("hex fixture drift for %s", sample.Name)
			}
		})
	}
}

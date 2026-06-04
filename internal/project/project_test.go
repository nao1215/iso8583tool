package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesWorkspace(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "workspace")
	result, err := Init(root, "demo")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	paths := []string{
		result.ConfigPath,
		filepath.Join(result.SpecsDir, "extensions.json"),
		filepath.Join(result.ExamplesDir, "basei", "0100-auth-request.json"),
		filepath.Join(result.ExamplesDir, "basei", "0100-auth-request.hex"),
		filepath.Join(result.ExamplesDir, "basei", "0110-auth-response.json"),
		filepath.Join(result.ExamplesDir, "basei", "0110-auth-response.hex"),
		result.MessagesDir,
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	loadedRoot, cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loadedRoot != root {
		t.Fatalf("Load root = %q, want %q", loadedRoot, root)
	}
	if cfg.Project.Name != "demo" {
		t.Fatalf("cfg.Project.Name = %q, want demo", cfg.Project.Name)
	}
}

package messagespec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moov-io/iso8583"
	moovspecs "github.com/moov-io/iso8583/specs"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
)

type Spec struct {
	MessageSpec *iso8583.MessageSpec
	Label       string
}

// Load resolves the message spec from a config. An empty value selects the
// default preset; a value matching a built-in preset uses that preset; any
// other value is treated as a path to a moov-io/iso8583 JSON spec, resolved
// relative to baseDir.
func Load(baseDir string, cfg config.Config) (*Spec, error) {
	spec := strings.TrimSpace(cfg.Spec)
	if spec == "" {
		spec = basei.StarterPreset
	}
	if preset, ok := basei.LookupPreset(spec); ok {
		return &Spec{
			MessageSpec: preset.Spec(),
			Label:       preset.Name,
		}, nil
	}
	return loadJSONSpec(baseDir, spec)
}

func loadJSONSpec(baseDir, path string) (*Spec, error) {
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(baseDir, path)
	}
	if ext := strings.ToLower(filepath.Ext(resolved)); ext != ".json" {
		return nil, fmt.Errorf("unsupported spec file %q: only JSON is supported", path)
	}

	data, err := os.ReadFile(filepath.Clean(resolved))
	if err != nil {
		return nil, err
	}
	messageSpec, err := moovspecs.ImportJSON(data)
	if err != nil {
		return nil, err
	}
	return &Spec{
		MessageSpec: messageSpec,
		Label:       resolved,
	}, nil
}

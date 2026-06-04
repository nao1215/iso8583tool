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

func Load(root string, cfg config.Config) (*Spec, error) {
	if path := strings.TrimSpace(cfg.Spec.MessageSpec); path != "" {
		return loadJSONSpec(root, path)
	}

	switch strings.TrimSpace(cfg.Spec.Preset) {
	case "", basei.StarterPreset:
		return &Spec{
			MessageSpec: basei.StarterMessageSpec(),
			Label:       "basei-starter",
		}, nil
	case "spec87ascii":
		return &Spec{
			MessageSpec: moovspecs.Spec87ASCII,
			Label:       "spec87ascii",
		}, nil
	default:
		return nil, fmt.Errorf("unknown preset %q", cfg.Spec.Preset)
	}
}

func loadJSONSpec(root, path string) (*Spec, error) {
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, path)
	}
	if ext := strings.ToLower(filepath.Ext(resolved)); ext != ".json" {
		return nil, fmt.Errorf("unsupported spec file %q: only JSON is supported in the scaffold", path)
	}

	data, err := os.ReadFile(filepath.Clean(resolved))
	if err != nil {
		return nil, err
	}
	messageSpec, err := moovspecs.Builder.ImportJSON(data)
	if err != nil {
		return nil, err
	}
	return &Spec{
		MessageSpec: messageSpec,
		Label:       resolved,
	}, nil
}

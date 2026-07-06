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
		// A bare word with no path separator and no extension is almost certainly
		// a mistyped preset name, not a file; point the user at `specs` instead of
		// telling them their preset "is not JSON".
		if !strings.ContainsAny(path, `/\`) && filepath.Ext(path) == "" {
			return nil, fmt.Errorf("unknown spec %q: not a built-in preset and not a .json spec path; run \"iso8583tool specs\" to list presets, or pass a moov-io/iso8583 JSON spec path", path)
		}
		return nil, fmt.Errorf("unsupported spec file %q: only JSON specs are supported (a .json path) or a built-in preset name (run \"iso8583tool specs\")", path)
	}

	data, err := os.ReadFile(filepath.Clean(resolved))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("spec file not found: %s", path)
		}
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

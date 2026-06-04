package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Project   Project
	Spec      Spec
	IO        IO
	Transport Transport
}

type Project struct {
	Name string
}

type Spec struct {
	Preset           string
	MessageSpec      string
	ExtensionCatalog string
}

type IO struct {
	InputEncoding  string
	OutputEncoding string
}

type Transport struct {
	Header string
}

func Default(projectName string) Config {
	return Config{
		Project: Project{
			Name: strings.TrimSpace(projectName),
		},
		Spec: Spec{
			Preset:           "basei-starter",
			MessageSpec:      "",
			ExtensionCatalog: "./specs/extensions.json",
		},
		IO: IO{
			InputEncoding:  "hex",
			OutputEncoding: "hex",
		},
		Transport: Transport{
			Header: "none",
		},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, err
	}
	return Parse(data)
}

func Parse(data []byte) (Config, error) {
	cfg := Default("iso8583tool")
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("invalid config line: %s", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch section {
		case "project":
			if key == "name" {
				parsed, err := parseString(value)
				if err != nil {
					return Config{}, err
				}
				cfg.Project.Name = parsed
			}
		case "spec":
			parsed, err := parseString(value)
			if err != nil {
				return Config{}, err
			}
			switch key {
			case "preset":
				cfg.Spec.Preset = parsed
			case "message_spec":
				cfg.Spec.MessageSpec = parsed
			case "extension_catalog":
				cfg.Spec.ExtensionCatalog = parsed
			}
		case "io":
			parsed, err := parseString(value)
			if err != nil {
				return Config{}, err
			}
			switch key {
			case "input_encoding":
				cfg.IO.InputEncoding = parsed
			case "output_encoding":
				cfg.IO.OutputEncoding = parsed
			}
		case "transport":
			if key == "header" {
				parsed, err := parseString(value)
				if err != nil {
					return Config{}, err
				}
				cfg.Transport.Header = parsed
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	return os.WriteFile(filepath.Clean(path), []byte(cfg.String()), 0o600)
}

func (c Config) String() string {
	return fmt.Sprintf(`[project]
name = %q

[spec]
preset = %q
message_spec = %q
extension_catalog = %q

[io]
input_encoding = %q
output_encoding = %q

[transport]
header = %q
`,
		c.Project.Name,
		c.Spec.Preset,
		c.Spec.MessageSpec,
		c.Spec.ExtensionCatalog,
		c.IO.InputEncoding,
		c.IO.OutputEncoding,
		c.Transport.Header,
	)
}

func parseString(raw string) (string, error) {
	if strings.HasPrefix(raw, "\"") {
		return strconv.Unquote(raw)
	}
	return raw, nil
}

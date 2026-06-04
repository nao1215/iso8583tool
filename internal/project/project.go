package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/service"
)

const ConfigFileName = "iso8583tool.toml"

var ErrProjectNotFound = errors.New("iso8583tool project not found")

type InitResult struct {
	Root        string
	ConfigPath  string
	SpecsDir    string
	ExamplesDir string
	MessagesDir string
}

func Init(root string, name string) (InitResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return InitResult{}, err
	}
	if err := os.MkdirAll(absRoot, 0o750); err != nil {
		return InitResult{}, err
	}

	configPath := filepath.Join(absRoot, ConfigFileName)
	if _, err := os.Stat(configPath); err == nil {
		return InitResult{}, fmt.Errorf("%s already exists", configPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return InitResult{}, err
	}

	projectName := strings.TrimSpace(name)
	if projectName == "" {
		projectName = filepath.Base(absRoot)
	}
	cfg := config.Default(projectName)

	specsDir := filepath.Join(absRoot, "specs")
	examplesDir := filepath.Join(absRoot, "examples")
	messagesDir := filepath.Join(absRoot, "messages")
	if err := os.MkdirAll(specsDir, 0o750); err != nil {
		return InitResult{}, err
	}
	if err := os.MkdirAll(examplesDir, 0o750); err != nil {
		return InitResult{}, err
	}
	if err := os.MkdirAll(messagesDir, 0o750); err != nil {
		return InitResult{}, err
	}
	if err := config.Save(configPath, cfg); err != nil {
		return InitResult{}, err
	}
	if err := basei.SaveCatalog(filepath.Join(specsDir, "extensions.json"), basei.DefaultExtensionCatalog()); err != nil {
		return InitResult{}, err
	}
	if err := writeSamples(examplesDir); err != nil {
		return InitResult{}, err
	}

	return InitResult{
		Root:        absRoot,
		ConfigPath:  configPath,
		SpecsDir:    specsDir,
		ExamplesDir: examplesDir,
		MessagesDir: messagesDir,
	}, nil
}

func writeSamples(examplesDir string) error {
	baseiDir := filepath.Join(examplesDir, "basei")
	for _, sample := range basei.StarterSamples() {
		docPath := filepath.Join(baseiDir, sample.Name+".json")
		if err := messageio.SaveDocument(docPath, sample.Document); err != nil {
			return err
		}

		packed, err := service.WriteMessage(sample.Document, basei.StarterMessageSpec())
		if err != nil {
			return err
		}
		encoded, err := messageio.EncodeOutput(packed.Raw, "hex")
		if err != nil {
			return err
		}
		hexPath := filepath.Join(baseiDir, sample.Name+".hex")
		if err := messageio.SaveBytes(hexPath, append(encoded, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func Load(start string) (string, config.Config, error) {
	root, err := FindRoot(start)
	if err != nil {
		return "", config.Config{}, err
	}
	cfg, err := config.Load(filepath.Join(root, ConfigFileName))
	if err != nil {
		return "", config.Config{}, err
	}
	return root, cfg, nil
}

func LoadOptional(start string) (string, config.Config, bool, error) {
	root, err := FindRoot(start)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return "", config.Config{}, false, nil
		}
		return "", config.Config{}, false, err
	}
	cfg, err := config.Load(filepath.Join(root, ConfigFileName))
	if err != nil {
		return "", config.Config{}, false, err
	}
	return root, cfg, true, nil
}

func FindRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		configPath := filepath.Join(current, ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrProjectNotFound
		}
		current = parent
	}
}

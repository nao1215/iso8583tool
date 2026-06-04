package messageio

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Document struct {
	MTI          string            `json:"mti"`
	Fields       map[string]string `json:"fields,omitempty"`
	BinaryFields map[string]string `json:"binary_fields,omitempty"`
}

func LoadDocument(path string) (Document, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Document{}, err
	}
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return Document{}, err
	}
	if err := doc.Validate(); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func SaveDocument(path string, doc Document) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Clean(path), data, 0o600)
}

func SaveBytes(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(path), data, 0o600)
}

func (d Document) Validate() error {
	if d.MTI == "" {
		return errors.New("message document requires mti")
	}
	return nil
}

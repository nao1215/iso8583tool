package messageio

import (
	"encoding/json"
	"errors"
)

type Document struct {
	MTI          string            `json:"mti"`
	Fields       map[string]string `json:"fields,omitempty"`
	BinaryFields map[string]string `json:"binary_fields,omitempty"`
}

// ParseDocument decodes and validates a message document from JSON bytes.
func ParseDocument(data []byte) (Document, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return Document{}, err
	}
	if err := doc.Validate(); err != nil {
		return Document{}, err
	}
	return doc, nil
}

// LooksLikeJSON reports whether the raw input is a JSON document (used to pick
// the convert direction).
func LooksLikeJSON(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

func (d Document) Validate() error {
	if d.MTI == "" {
		return errors.New("message document requires mti")
	}
	return nil
}

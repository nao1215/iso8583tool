package messageio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// utf8BOM is the UTF-8 byte order mark. Editors and some exporters prepend it to
// a JSON file; it is not valid JSON and breaks both detection and decoding, so
// it is stripped before either.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type Document struct {
	MTI          string            `json:"mti"`
	Fields       map[string]string `json:"fields,omitempty"`
	BinaryFields map[string]string `json:"binary_fields,omitempty"`
}

// ParseDocument decodes and validates a message document from JSON bytes.
func ParseDocument(data []byte) (Document, error) {
	data = bytes.TrimPrefix(data, utf8BOM)
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
	data = bytes.TrimPrefix(data, utf8BOM)
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
	return d.validatePaths()
}

// validatePaths rejects ambiguous documents: a path that appears in both
// fields and binary_fields, or a parent path that also has nested children
// (for example "55" together with "55.9F02", or "48" with "48.1"). Either form
// makes packing order-dependent and silently lossy, so it must fail fast.
func (d Document) validatePaths() error {
	owner := make(map[string]string, len(d.Fields)+len(d.BinaryFields))
	for p := range d.Fields {
		owner[p] = "fields"
	}
	for p := range d.BinaryFields {
		if where, ok := owner[p]; ok {
			return fmt.Errorf("path %q is defined in both %s and binary_fields; keep it in only one", p, where)
		}
		owner[p] = "binary_fields"
	}

	paths := make([]string, 0, len(owner))
	for p := range owner {
		paths = append(paths, p)
	}
	sort.Strings(paths) // a parent path sorts before its dotted children
	for i, parent := range paths {
		for _, child := range paths[i+1:] {
			if strings.HasPrefix(child, parent+".") {
				return fmt.Errorf("path %q conflicts with nested path %q; set the parent field or its children, not both", parent, child)
			}
		}
	}
	return nil
}

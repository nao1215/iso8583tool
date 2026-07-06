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
	if err := validateMTI(d.MTI); err != nil {
		return err
	}
	return d.validatePaths()
}

// validateMTI rejects a message type indicator that is not exactly four decimal
// digits. An ISO 8583 MTI is a 4-digit numeric code (e.g. 0100, 0810); catching
// a malformed one here yields a plain "MTI must be exactly 4 digits" instead of
// moov's cryptic pack-time internals ("field length: 2 should be fixed: 4") or,
// worse, silently packing an MTI that no reader can interpret.
func validateMTI(mti string) error {
	if len(mti) != 4 || !isAllDigits(mti) {
		return fmt.Errorf("mti must be exactly 4 digits, got %q", mti)
	}
	return nil
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// pathEntry records the original spelling and source map of a canonical path so
// validation errors can point back at exactly what the document wrote.
type pathEntry struct {
	raw   string
	where string
}

// validatePaths rejects ambiguous or malformed documents. It fails fast on:
//   - syntactically invalid paths (reserved id, out-of-range id, whitespace,
//     empty segments) — see canonicalPath;
//   - two spellings of the same field ("2" and "02", "55.9f02" and "55.9F02");
//   - a path present in both fields and binary_fields;
//   - a parent path that also has nested children ("55" with "55.9F02").
//
// Every one of these makes packing order-dependent and silently lossy, so the
// document must not pack at all.
func (d Document) validatePaths() error {
	owner := make(map[string]pathEntry, len(d.Fields)+len(d.BinaryFields))
	register := func(raw, where string) error {
		canon, err := canonicalPath(raw)
		if err != nil {
			return err
		}
		if prev, ok := owner[canon]; ok {
			if prev.where != where {
				return fmt.Errorf("path %q is defined in both %s and %s; keep it in only one", raw, prev.where, where)
			}
			return fmt.Errorf("paths %q and %q both address field %q; keep only one", prev.raw, raw, canon)
		}
		owner[canon] = pathEntry{raw: raw, where: where}
		return nil
	}
	for p := range d.Fields {
		if err := register(p, "fields"); err != nil {
			return err
		}
	}
	for p := range d.BinaryFields {
		if err := register(p, "binary_fields"); err != nil {
			return err
		}
	}

	canons := make([]string, 0, len(owner))
	for c := range owner {
		canons = append(canons, c)
	}
	sort.Strings(canons) // a parent path sorts before its dotted children
	for i, parent := range canons {
		for _, child := range canons[i+1:] {
			if strings.HasPrefix(child, parent+".") {
				return fmt.Errorf("path %q conflicts with nested path %q; set the parent field or its children, not both", owner[parent].raw, owner[child].raw)
			}
		}
	}
	return nil
}

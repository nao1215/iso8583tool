package service

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/field"

	"github.com/nao1215/iso8583tool/internal/annotate"
	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/render"
)

type ViewResult struct {
	Body        string
	Summary     string
	Extensions  []basei.ExtensionField
	UnknownTags []UnknownTag
	Decoded     []DecodedField
}

// DecodedField is a coded value translated into a human meaning.
type DecodedField struct {
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value"`
	Meaning     string `json:"meaning,omitempty"`
}

func ViewMessage(raw []byte, spec *iso8583.MessageSpec, catalog basei.ExtensionCatalog, format string, filters []string, pal render.Palette) (ViewResult, error) {
	msg := iso8583.NewMessage(spec)
	if err := safeUnpack(msg, raw); err != nil {
		return ViewResult{}, errors.New(diagnoseUnpack(err, raw).String())
	}

	unknownTags := collectUnknownTags(msg)
	extensions := activeExtensions(msg.GetFields(), catalog)
	decoded := DecodeFields(msg)
	summary := Summarize(msg)

	if len(filters) > 0 {
		body, err := renderFiltered(msg, filters, format, pal)
		if err != nil {
			return ViewResult{}, err
		}
		return ViewResult{Body: body, Summary: summary, Decoded: decoded}, nil
	}

	switch format {
	case "", "describe", "text":
		var buf bytes.Buffer
		if err := iso8583.Describe(msg, &buf); err != nil {
			return ViewResult{}, err
		}
		body := colorizeDescribe(buf.String(), pal)
		return ViewResult{
			Body:        body,
			Summary:     summary,
			Extensions:  extensions,
			UnknownTags: unknownTags,
			Decoded:     decoded,
		}, nil
	case "json":
		// Document-shaped, jq-friendly output: mti/fields/binary_fields share the
		// same representation as convert, redact and diff. Map keys are sorted by
		// encoding/json, so the output is deterministic and scriptable.
		doc, err := MessageToDocument(spec, raw)
		if err != nil {
			return ViewResult{}, err
		}
		payload := struct {
			MTI          string                 `json:"mti"`
			Fields       map[string]string      `json:"fields,omitempty"`
			BinaryFields map[string]string      `json:"binary_fields,omitempty"`
			Summary      string                 `json:"summary,omitempty"`
			Extensions   []basei.ExtensionField `json:"extension_fields,omitempty"`
			UnknownTags  []UnknownTag           `json:"unknown_tags,omitempty"`
			Decoded      []DecodedField         `json:"decoded,omitempty"`
		}{
			MTI:          doc.MTI,
			Fields:       doc.Fields,
			BinaryFields: doc.BinaryFields,
			Summary:      summary,
			Extensions:   extensions,
			UnknownTags:  unknownTags,
			Decoded:      decoded,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return ViewResult{}, err
		}
		return ViewResult{
			Body:        string(data),
			Summary:     summary,
			Extensions:  extensions,
			UnknownTags: unknownTags,
			Decoded:     decoded,
		}, nil
	default:
		return ViewResult{}, fmt.Errorf("unsupported view format %q", format)
	}
}

// Summarize builds a one-line, human-readable digest of a message.
func Summarize(msg *iso8583.Message) string {
	fields := msg.GetFields()
	get := func(id int) (string, bool) {
		f, ok := fields[id]
		if !ok {
			return "", false
		}
		s, err := f.String()
		if err != nil {
			return "", false
		}
		return s, true
	}

	var parts []string
	if mti, err := msg.GetMTI(); err == nil && mti != "" {
		parts = append(parts, mti)
	}
	if v, ok := get(39); ok {
		if m, ok := annotate.FieldMeaning("39", v); ok {
			parts = append(parts, m)
		}
	}
	if v, ok := get(70); ok {
		if m, ok := annotate.FieldMeaning("70", v); ok {
			parts = append(parts, m)
		}
	}
	if amount, ok := get(4); ok {
		currency, _ := get(49)
		if s, ok := annotate.FormatAmount(amount, currency); ok {
			parts = append(parts, s)
		} else if trimmed := strings.TrimLeft(amount, "0"); trimmed != "" {
			parts = append(parts, "amount "+trimmed)
		}
	}
	if v, ok := get(11); ok {
		parts = append(parts, "STAN "+v)
	}
	if v, ok := get(41); ok && strings.TrimSpace(v) != "" {
		parts = append(parts, strings.TrimSpace(v))
	}
	return strings.Join(parts, " · ")
}

// lookupPath resolves a dot-path to its leaf field, returning the spec
// description and string value.
func lookupPath(msg *iso8583.Message, path string) (description, value string, ok bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", "", false
	}
	f, present := msg.GetFields()[id]
	if !present {
		return "", "", false
	}
	for _, part := range parts[1:] {
		container, ok := f.(interface {
			GetSubfields() map[string]field.Field
		})
		if !ok {
			return "", "", false
		}
		sub, present := container.GetSubfields()[part]
		if !present {
			return "", "", false
		}
		f = sub
	}
	str, err := f.String()
	if err != nil {
		return "", "", false
	}
	return f.Spec().Description, str, true
}

// renderFiltered renders only the requested field paths.
func renderFiltered(msg *iso8583.Message, filters []string, format string, pal render.Palette) (string, error) {
	matched := make([]DecodedField, 0, len(filters))
	var missing []string
	for _, path := range filters {
		desc, value, ok := lookupPath(msg, path)
		if !ok {
			missing = append(missing, path)
			continue
		}
		entry := DecodedField{Path: path, Description: desc, Value: value}
		if meaning, ok := annotate.FieldMeaning(path, strings.TrimSpace(value)); ok {
			entry.Meaning = meaning
		}
		matched = append(matched, entry)
	}

	if format == "json" {
		data, err := json.MarshalIndent(matched, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var b strings.Builder
	for _, m := range matched {
		line := pal.Green("F"+m.Path) + " " + m.Description + ": " + pal.Yellow(m.Value)
		if m.Meaning != "" {
			line += "  " + pal.Cyan("→ "+m.Meaning)
		}
		b.WriteString(line + "\n")
	}
	for _, path := range missing {
		b.WriteString(pal.Dim("F"+path+": <not present>") + "\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// DecodeFields walks the present fields and returns the ones whose coded value
// maps to a human meaning (MTI, response code, currency, EMV tags, ...).
func DecodeFields(msg *iso8583.Message) []DecodedField {
	var decoded []DecodedField

	if mti, err := msg.GetMTI(); err == nil && mti != "" {
		if meaning := annotate.MTI(mti); meaning != "" {
			decoded = append(decoded, DecodedField{Path: "0", Value: mti, Meaning: meaning})
		}
	}

	fields := msg.GetFields()
	ids := make([]int, 0, len(fields))
	for id := range fields {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		path := fmt.Sprintf("%d", id)
		f := fields[id]
		if composite, ok := f.(interface {
			GetSubfields() map[string]field.Field
		}); ok {
			decoded = append(decoded, decodeSubfields(path, composite.GetSubfields())...)
			continue
		}
		value, err := f.String()
		if err != nil {
			continue
		}
		if meaning, ok := annotate.FieldMeaning(path, value); ok {
			decoded = append(decoded, DecodedField{Path: path, Value: value, Meaning: meaning})
		}
	}
	return decoded
}

func decodeSubfields(parent string, subfields map[string]field.Field) []DecodedField {
	tags := make([]string, 0, len(subfields))
	for tag := range subfields {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	var decoded []DecodedField
	for _, tag := range tags {
		value, err := subfields[tag].String()
		if err != nil {
			continue
		}
		path := parent + "." + tag
		if meaning, ok := annotate.FieldMeaning(path, value); ok {
			decoded = append(decoded, DecodedField{Path: path, Value: value, Meaning: meaning})
		}
	}
	return decoded
}

// colorizeDescribe post-processes the plain moov describe output, adding color
// and inline meaning annotations while preserving its layout (and masking).
func colorizeDescribe(plain string, pal render.Palette) string {
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	parentPrefix := ""
	dashesSeen := 0

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasSuffix(line, "Message:"):
			out = append(out, pal.BoldCyan(line))
		case isDashes(line):
			if parentPrefix != "" {
				dashesSeen++
				if dashesSeen >= 2 {
					parentPrefix = ""
					dashesSeen = 0
				}
			}
			out = append(out, pal.Dim(line))
		case strings.HasPrefix(line, "Bitmap"):
			out = append(out, colorizeKeyValue(line, pal, pal.Green))
		case strings.HasPrefix(line, "MTI"):
			out = append(out, colorizeMTILine(line, pal))
		case strings.HasPrefix(line, "F") && strings.Contains(line, "SUBFIELDS:"):
			parentPrefix = fieldID(line)
			dashesSeen = 0
			out = append(out, pal.Magenta(line))
		case strings.HasPrefix(line, "F") && strings.Contains(line, ": "):
			out = append(out, colorizeFieldLine(line, parentPrefix, pal))
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func isDashes(line string) bool {
	if line == "" {
		return false
	}
	return strings.Trim(line, "-") == ""
}

// fieldID extracts the numeric/hex id token from a line like "F55  ... ".
func fieldID(line string) string {
	token := strings.Fields(line)
	if len(token) == 0 {
		return ""
	}
	return strings.TrimPrefix(token[0], "F")
}

func colorizeKeyValue(line string, pal render.Palette, valueColor func(string) string) string {
	idx := strings.Index(line, ": ")
	if idx < 0 {
		return line
	}
	key := line[:idx]
	value := line[idx+2:]
	return pal.Dim(key) + ": " + valueColor(value)
}

func colorizeMTILine(line string, pal render.Palette) string {
	idx := strings.Index(line, ": ")
	if idx < 0 {
		return line
	}
	key := line[:idx]
	value := strings.TrimSpace(line[idx+2:])
	rendered := pal.Dim(key) + ": " + pal.BoldGreen(value)
	if meaning := annotate.MTI(value); meaning != "" {
		rendered += "  " + pal.Cyan("→ "+meaning)
	}
	return rendered
}

func colorizeFieldLine(line, parentPrefix string, pal render.Palette) string {
	idx := strings.Index(line, ": ")
	if idx < 0 {
		return line
	}
	label := line[:idx]
	value := line[idx+2:]

	id := fieldID(label)
	path := id
	coloredLabel := label
	token := strings.Fields(label)

	if parentPrefix != "" && len(token) > 0 {
		// Subfield: show the full dot-path (e.g. 55.9F26) instead of moov's
		// "F9F26", trimming alignment dots so the value column stays put.
		path = parentPrefix + "." + id
		rest := label[len(token[0]):]
		if delta := len(path) - len(token[0]); delta > 0 {
			rest = trimTrailingDots(rest, delta)
		}
		coloredLabel = pal.Green(path) + rest
	} else if len(token) > 0 {
		coloredLabel = strings.Replace(label, token[0], pal.Green(token[0]), 1)
	}

	rendered := coloredLabel + ": " + pal.Yellow(value)
	if meaning, ok := annotate.FieldMeaning(path, strings.TrimSpace(value)); ok {
		rendered += "  " + pal.Cyan("→ "+meaning)
	}
	return rendered
}

// trimTrailingDots removes up to n trailing '.' runes from s.
func trimTrailingDots(s string, n int) string {
	for n > 0 && strings.HasSuffix(s, ".") {
		s = s[:len(s)-1]
		n--
	}
	return s
}

func activeExtensions(fields map[int]field.Field, catalog basei.ExtensionCatalog) []basei.ExtensionField {
	ids := make([]int, 0, len(fields))
	for id := range fields {
		if _, ok := catalog.Lookup(id); ok {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)

	result := make([]basei.ExtensionField, 0, len(ids))
	for _, id := range ids {
		fieldDef, ok := catalog.Lookup(id)
		if ok {
			result = append(result, fieldDef)
		}
	}
	return result
}

type UnknownTag struct {
	Path string `json:"path"`
	Raw  string `json:"raw"`
}

func collectUnknownTags(msg *iso8583.Message) []UnknownTag {
	unknown := iso8583.UnknownTags(msg)
	if len(unknown) == 0 {
		return nil
	}

	paths := make([]string, 0, len(unknown))
	for path := range unknown {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	result := make([]UnknownTag, 0, len(paths))
	for _, path := range paths {
		raw := ""
		if data, err := unknown[path].Bytes(); err == nil {
			raw = strings.ToUpper(hex.EncodeToString(data))
		}
		result = append(result, UnknownTag{
			Path: path,
			Raw:  raw,
		})
	}
	return result
}

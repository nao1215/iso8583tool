package service

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/field"

	"github.com/nao1215/iso8583tool/internal/annotate"
	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/render"
)

type ViewResult struct {
	Body        string
	Extensions  []basei.ExtensionField
	UnknownTags []UnknownTag
	Decoded     []DecodedField
}

// DecodedField is a coded value translated into a human meaning.
type DecodedField struct {
	Path    string `json:"path"`
	Value   string `json:"value"`
	Meaning string `json:"meaning"`
}

func ViewMessage(raw []byte, spec *iso8583.MessageSpec, catalog basei.ExtensionCatalog, format string, pal render.Palette) (ViewResult, error) {
	msg := iso8583.NewMessage(spec)
	if err := msg.Unpack(raw); err != nil {
		return ViewResult{}, err
	}

	unknownTags := collectUnknownTags(msg)
	extensions := activeExtensions(msg.GetFields(), catalog)
	decoded := DecodeFields(msg)

	switch format {
	case "", "describe", "text":
		var buf bytes.Buffer
		if err := iso8583.Describe(msg, &buf); err != nil {
			return ViewResult{}, err
		}
		body := colorizeDescribe(buf.String(), pal)
		return ViewResult{
			Body:        body,
			Extensions:  extensions,
			UnknownTags: unknownTags,
			Decoded:     decoded,
		}, nil
	case "json":
		payload := struct {
			Message     *iso8583.Message       `json:"message"`
			Extensions  []basei.ExtensionField `json:"extension_fields,omitempty"`
			UnknownTags []UnknownTag           `json:"unknown_tags,omitempty"`
			Decoded     []DecodedField         `json:"decoded,omitempty"`
		}{
			Message:     msg,
			Extensions:  extensions,
			UnknownTags: unknownTags,
			Decoded:     decoded,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return ViewResult{}, err
		}
		return ViewResult{
			Body:        string(data),
			Extensions:  extensions,
			UnknownTags: unknownTags,
			Decoded:     decoded,
		}, nil
	default:
		return ViewResult{}, fmt.Errorf("unsupported view format %q", format)
	}
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
	if parentPrefix != "" {
		path = parentPrefix + "." + id
	}

	coloredLabel := label
	if token := strings.Fields(label); len(token) > 0 {
		coloredLabel = strings.Replace(label, token[0], pal.Green(token[0]), 1)
	}

	rendered := coloredLabel + ": " + pal.Yellow(value)
	if meaning, ok := annotate.FieldMeaning(path, strings.TrimSpace(value)); ok {
		rendered += "  " + pal.Cyan("→ "+meaning)
	}
	return rendered
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

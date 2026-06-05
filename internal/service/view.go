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
	"github.com/nao1215/iso8583tool/internal/messageio"
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

func ViewMessage(raw []byte, spec *iso8583.MessageSpec, catalog basei.ExtensionCatalog, format string, filters []string, pal render.Palette, unsafe bool) (ViewResult, error) {
	msg := iso8583.NewMessage(spec)
	if err := safeUnpack(msg, raw); err != nil {
		return ViewResult{}, errors.New(diagnoseUnpack(err, raw).String())
	}

	unknownTags := collectUnknownTags(msg)
	extensions := activeExtensions(msg.GetFields(), catalog)
	decoded := DecodeFields(msg)
	summary := Summarize(msg)

	// Unknown TLV tags carry partner-defined data the spec cannot vouch for
	// (e.g. an unmapped Track 2 tag holding a PAN), so every display surface
	// masks their bytes by default. --unsafe opts into the raw values, and
	// convert always keeps them intact for round-trip safety.
	displayUnknownTags := maskUnknownTagValues(unknownTags)
	if unsafe {
		displayUnknownTags = unknownTags
	}

	// maskForDisplay applies the default masking unless the caller opted into raw
	// output, so the filtered and json branches share one masking decision.
	builtinSemantics := basei.IsBuiltinMessageSpec(spec)
	maskForDisplay := func(doc *messageio.Document) {
		if unsafe {
			return
		}
		MaskCardholderData(doc, builtinSemantics)
		maskUnknownInDocument(doc, unknownTags)
	}

	if len(filters) > 0 {
		doc, err := MessageToDocument(spec, raw)
		if err != nil {
			return ViewResult{}, err
		}
		maskForDisplay(&doc)
		body, err := renderFiltered(msg, doc, filters, format, pal, summary)
		if err != nil {
			return ViewResult{}, err
		}
		return ViewResult{Body: body, Summary: summary, Decoded: decoded}, nil
	}

	switch format {
	case "", "describe", "text":
		var buf bytes.Buffer
		describeFilters := safeDescribeFilters(msg)
		if unsafe {
			describeFilters = iso8583.DoNotFilterFields()
		}
		if err := iso8583.Describe(msg, &buf, describeFilters...); err != nil {
			return ViewResult{}, err
		}
		body := buf.String()
		// Sensitive masking is applied per dot-path in colorizeDescribe (so a
		// top-level field is masked without touching a same-numbered subfield);
		// unknown-tag bytes are masked here since they are not field lines.
		var maskFn func(path, value string) string
		if !unsafe {
			body = maskUnknownInText(body, unknownTags)
			maskFn = func(path, value string) string { return maskValueForDiff(path, value, nil, builtinSemantics) }
		}
		body = colorizeDescribe(body, pal, maskFn)
		return ViewResult{
			Body:        body,
			Summary:     summary,
			Extensions:  extensions,
			UnknownTags: displayUnknownTags,
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
		maskForDisplay(&doc)
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
			UnknownTags:  displayUnknownTags,
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
			UnknownTags: displayUnknownTags,
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

// renderFiltered renders only the requested field paths from the (already
// masked) document. A filter on a composite root expands into its child paths,
// matching diff, instead of dumping the composite's raw bytes.
func renderFiltered(msg *iso8583.Message, doc messageio.Document, filters []string, format string, pal render.Palette, summary string) (string, error) {
	flat := FlattenDocument(doc)
	// The MTI is rendered top-level in the document shape, never as a field
	// path, so drop the pseudo-path FlattenDocument injects before it can leak
	// into "fields". It is selected separately, by field 0 or "mti".
	delete(flat, "mti")
	paths := make([]string, 0, len(flat))
	for p := range flat {
		paths = append(paths, p)
	}
	sortPaths(paths)

	matchedFilter := make(map[string]bool, len(filters))
	matched := make([]DecodedField, 0, len(paths))
	for _, path := range paths {
		f := matchingFilter(path, filters)
		if f == "" {
			continue
		}
		matchedFilter[f] = true
		value := flat[path]
		desc, _, _ := lookupPath(msg, path)
		entry := DecodedField{Path: path, Description: desc, Value: value}
		if meaning, ok := annotate.FieldMeaning(path, strings.TrimSpace(value)); ok {
			entry.Meaning = meaning
		}
		matched = append(matched, entry)
	}

	// The MTI is ISO 8583 field 0; "0" or "mti" selects it. It stays top-level
	// (matching the document shape) and only carries its decoded meaning here,
	// so it is never reported missing nor duplicated into "fields".
	mtiSelected := false
	mtiEntry := DecodedField{Path: "0", Value: doc.MTI}
	if doc.MTI != "" {
		for _, f := range filters {
			if u := strings.ToUpper(strings.TrimSpace(f)); u == "0" || u == "MTI" {
				matchedFilter[f] = true
				mtiSelected = true
			}
		}
		if meaning := annotate.MTI(doc.MTI); meaning != "" {
			mtiEntry.Meaning = meaning
		}
	}

	missing := make([]string, 0, len(filters))
	for _, f := range filters {
		if !matchedFilter[f] {
			missing = append(missing, f)
		}
	}

	if format == "json" {
		// A consistent subset of the unfiltered `view --format json`: the same
		// mti / fields / binary_fields / summary / decoded keys, scoped to the
		// matched paths, plus an always-present missing_filters array so a typo
		// or an absent field is distinguishable in a stable shape.
		fieldsOut := map[string]string{}
		binaryOut := map[string]string{}
		var decodedOut []DecodedField
		if mtiSelected && mtiEntry.Meaning != "" {
			decodedOut = append(decodedOut, mtiEntry)
		}
		for _, m := range matched {
			if _, ok := doc.BinaryFields[m.Path]; ok {
				binaryOut[m.Path] = m.Value
			} else {
				fieldsOut[m.Path] = m.Value
			}
			if m.Meaning != "" {
				decodedOut = append(decodedOut, DecodedField{Path: m.Path, Value: m.Value, Meaning: m.Meaning})
			}
		}
		payload := struct {
			MTI            string            `json:"mti"`
			Fields         map[string]string `json:"fields,omitempty"`
			BinaryFields   map[string]string `json:"binary_fields,omitempty"`
			Summary        string            `json:"summary,omitempty"`
			Decoded        []DecodedField    `json:"decoded,omitempty"`
			MissingFilters []string          `json:"missing_filters"`
		}{
			MTI:            doc.MTI,
			Summary:        summary,
			Decoded:        decodedOut,
			MissingFilters: missing,
		}
		if len(fieldsOut) > 0 {
			payload.Fields = fieldsOut
		}
		if len(binaryOut) > 0 {
			payload.BinaryFields = binaryOut
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var b strings.Builder
	if mtiSelected {
		line := pal.Green("MTI") + ": " + pal.Yellow(mtiEntry.Value)
		if mtiEntry.Meaning != "" {
			line += "  " + pal.Cyan("→ "+mtiEntry.Meaning)
		}
		b.WriteString(line + "\n")
	}
	for _, m := range matched {
		label := pal.Green("F" + m.Path)
		if m.Description != "" {
			label += " " + m.Description
		}
		line := label + ": " + pal.Yellow(m.Value)
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

// matchingFilter returns the filter that selects path, or "" when none does.
// Matching is case-insensitive on hex EMV tags so "55.9f02" selects "55.9F02".
func matchingFilter(path string, filters []string) string {
	for _, f := range filters {
		if pathSelectedByFilter(path, f) {
			return f
		}
	}
	return ""
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
		value = canonicalFieldValue(f, value)
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
		f := subfields[tag]
		path := parent + "." + tag
		// A constructed TLV tag (for example 55.70) is itself a composite; recurse
		// so its leaves (55.70.8A, ...) are decoded and annotated, not just the
		// parent's packed bytes.
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
		value = canonicalFieldValue(f, value)
		if meaning, ok := annotate.FieldMeaning(path, value); ok {
			decoded = append(decoded, DecodedField{Path: path, Value: value, Meaning: meaning})
		}
	}
	return decoded
}

// colorizeDescribe post-processes the plain moov describe output, adding color
// and inline meaning annotations while preserving its layout. When mask is
// non-nil it is applied to each field value by its full dot-path, so the text
// view masks the same paths as the JSON and diff views — and, crucially, masks
// a top-level field (for example PAN field 2) without touching a same-numbered
// composite subfield (48.2) that moov's id-keyed filters could not tell apart.
func colorizeDescribe(plain string, pal render.Palette, mask func(path, value string) string) string {
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	// stack holds the composite ids currently open, outermost first, so a leaf or
	// nested header reports its full dot-path (for example 55.70.9F02 or F48.2)
	// even several composites deep. moov delimits each composite with a dashes
	// line right after its header (an "open") and another at its end (a "close").
	var stack []string
	expectOpenDash := false

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasSuffix(line, "Message:"):
			out = append(out, pal.BoldCyan(line))
		case isDashes(line):
			if expectOpenDash {
				expectOpenDash = false
			} else if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			out = append(out, pal.Dim(line))
		case strings.HasPrefix(line, "Bitmap"):
			out = append(out, colorizeKeyValue(line, pal, pal.Green))
		case strings.HasPrefix(line, "MTI"):
			out = append(out, colorizeMTILine(line, pal))
		case strings.HasPrefix(line, "F") && strings.Contains(line, "SUBFIELDS:"):
			fullPath := fieldID(line)
			if len(stack) > 0 {
				fullPath = strings.Join(stack, ".") + "." + fullPath
			}
			stack = append(stack, fieldID(line))
			expectOpenDash = true
			out = append(out, colorizeSubfieldsHeader(line, fullPath, pal))
		case strings.HasPrefix(line, "F") && strings.Contains(line, ": "):
			out = append(out, colorizeFieldLine(line, strings.Join(stack, "."), pal, mask))
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// colorizeSubfieldsHeader rewrites a "F<localid> <desc> SUBFIELDS:" header so it
// shows the composite's full dot-path (for example "F48.2") instead of moov's
// local id, then colors it.
func colorizeSubfieldsHeader(line, fullPath string, pal render.Palette) string {
	token := strings.Fields(line)
	if len(token) == 0 {
		return pal.Magenta(line)
	}
	rest := strings.TrimLeft(line[len(token[0]):], " ")
	return pal.Magenta("F" + fullPath + " " + rest)
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

func colorizeFieldLine(line, parentPrefix string, pal render.Palette, mask func(path, value string) string) string {
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

	displayValue := value
	if mask != nil {
		displayValue = mask(path, value)
	}
	rendered := coloredLabel + ": " + pal.Yellow(displayValue)
	// Annotate from the original value: a masked value has no meaning, and a
	// non-sensitive field is returned unchanged by mask anyway.
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
		if !ok {
			continue
		}
		// Report the strategy the active spec actually uses, not the catalog's
		// BASE I assumption: a positional composite is positional, a bitmap
		// composite is bitmap, a BER-TLV composite is tlv, and a plain field is
		// opaque. This keeps a field documented as bitmap/positional but modeled
		// as a plain string from being mislabeled.
		fieldDef.Strategy = deriveStrategy(fields[id], fieldDef.Strategy)
		result = append(result, fieldDef)
	}
	return result
}

// deriveStrategy reports how the active spec models a field. A composite is
// classified by its tag/bitmap spec; any non-composite (plain string/numeric)
// is opaque. fallback is used only for the unusual composite with neither a tag
// nor a bitmap.
func deriveStrategy(f field.Field, fallback basei.ExtensionStrategy) basei.ExtensionStrategy {
	composite, ok := f.(*field.Composite)
	if !ok {
		return basei.StrategyOpaque
	}
	s := composite.Spec()
	switch {
	case s.Bitmap != nil:
		return basei.StrategyBitmap
	case s.Tag != nil && s.Tag.Enc != nil:
		return basei.StrategyTLV
	case s.Tag != nil:
		return basei.StrategyPositional
	default:
		return fallback
	}
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

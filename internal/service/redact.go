package service

import (
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/field"

	"github.com/nao1215/iso8583tool/internal/messageio"
)

// panFieldIDs are the positional fields that carry a primary account number:
// field 2 (PAN) and field 34 (Extended PAN). Field 20 is NOT here — in the 1987
// layout it is the "PAN Extended Country Code", not a secondary PAN.
var panFieldIDs = map[string]bool{"2": true, "34": true}

// trackFieldIDs are the positional fields that carry magnetic-stripe track data.
var trackFieldIDs = map[string]bool{"35": true, "36": true, "45": true}

// secretFieldIDs are positional fields that are fully masked (no BIN kept), such
// as the PIN block.
var secretFieldIDs = map[string]bool{"52": true}

// contentScanFieldIDs are reserved / private / additional-data fields whose
// free-form value can carry a PAN or track that no positional or per-tag rule
// would otherwise catch (for example "63":"PAN=4111..." or "44":"PAN=...").
var contentScanFieldIDs = map[string]bool{
	"43": true, "44": true, "46": true, "47": true, "48": true, "54": true,
	"60": true, "61": true, "62": true, "63": true,
	"120": true, "121": true, "122": true, "123": true,
	"124": true, "125": true, "126": true, "127": true,
}

// embeddedPANPattern matches a candidate PAN: 13-19 digits, optionally grouped
// by single spaces or hyphens (so "4111 1111 1111 1111" and "4111-1111-..."
// match too).
var embeddedPANPattern = regexp.MustCompile(`[0-9](?:[ -]?[0-9]){12,18}`)

// panKeyPattern matches a PAN-ish key label immediately before a candidate (for
// example "PAN=" or "card no: "), so an explicitly labeled account number is
// masked even when its digits are not Luhn-valid (some test PANs are not).
var panKeyPattern = regexp.MustCompile(`(?i)\b(pan|cardnumber|cardno|card|account|acct|cvv|cvc|cc)[\s:=]*$`)

// maskEmbeddedSensitive masks PAN-shaped runs inside a free-form value, keeping
// the BIN and last four and preserving any grouping separators. To avoid masking
// non-PAN identifiers (order ids, reference numbers), a candidate is masked only
// when its digits pass the Luhn check or it directly follows a PAN-ish key label;
// an arbitrary numeric id satisfies neither.
func maskEmbeddedSensitive(value string) string {
	locs := embeddedPANPattern.FindAllStringIndex(value, -1)
	if locs == nil {
		return value
	}
	var b strings.Builder
	prev := 0
	for _, loc := range locs {
		b.WriteString(value[prev:loc[0]])
		match := value[loc[0]:loc[1]]
		digits := digitsOnly(match)
		labeled := panKeyPattern.MatchString(value[:loc[0]])
		if len(digits) >= 13 && len(digits) <= 19 && (luhnValid(digits) || labeled) {
			b.WriteString(maskPANKeepingSeparators(match))
		} else {
			b.WriteString(match)
		}
		prev = loc[1]
	}
	b.WriteString(value[prev:])
	return b.String()
}

// isContentScanPath reports whether path's top-level field id is one whose
// free-form value should be content-scanned for an embedded PAN/track.
func isContentScanPath(path string) bool {
	return contentScanFieldIDs[topLevelID(path)]
}

// leafTag returns the trailing dot-segment of a composite path (the TLV tag),
// reporting false for a top-level positional field with no dot. So "55.57" and
// "55.70.57" both yield "57", while "57" (positional field 57) yields no tag.
func leafTag(path string) (string, bool) {
	i := strings.LastIndexByte(path, '.')
	if i < 0 {
		return "", false
	}
	return path[i+1:], true
}

// digitsOnly returns the ASCII digits of s, dropping separators.
func digitsOnly(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// luhnValid reports whether a digit string passes the Luhn checksum.
func luhnValid(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// maskPANKeepingSeparators masks all but the first six and last four digits of a
// PAN-shaped string, leaving any grouping separators in place.
func maskPANKeepingSeparators(s string) string {
	total := len(digitsOnly(s))
	out := []byte(s)
	seen := 0
	for i := 0; i < len(out); i++ {
		if out[i] < '0' || out[i] > '9' {
			continue
		}
		seen++
		if seen > 6 && seen <= total-4 {
			out[i] = '*'
		}
	}
	return string(out)
}

// binaryEmbedsSensitive reports whether the raw bytes behind a hex-encoded binary
// field value contain an embedded PAN/track when read as text. A binary private
// field can hold ASCII like "PAN=4111..." that the hex form hides.
func binaryEmbedsSensitive(hexValue string) bool {
	raw, err := hex.DecodeString(strings.ReplaceAll(hexValue, " ", ""))
	if err != nil {
		return false
	}
	s := string(raw)
	return maskEmbeddedSensitive(s) != s
}

// safeDescribeFilters returns moov Describe filters that mask the cardholder
// fields with the same length-preserving masks the JSON view uses. moov's own
// Track1/Track2/Track3 filters silently return the value unchanged when they
// cannot parse it as well-formed track data — and in the basei-starter spec the
// track fields are plain strings, so they never parse, which would leak full
// track data (PAN, expiry, discretionary) through the text view. Our masks do
// not depend on parsing, so they always mask.
//
// It also registers a canonical filter for every present primitive field so the
// describe output shows the same zero-padded width as the JSON and filtered
// views (moov's describe prints field.String(), which drops a fixed-length
// field's padding). The sensitive masks are appended last so they override the
// canonical filter for the fields they cover.
func safeDescribeFilters(msg *iso8583.Message) []iso8583.FieldFilter {
	mask := func(fn func(string) string) iso8583.FilterFunc {
		return func(in string, _ field.Field) string { return fn(in) }
	}
	canonical := func(in string, f field.Field) string { return canonicalFieldValue(f, in) }

	var filters []iso8583.FieldFilter
	for id := range msg.GetFields() {
		if id == 0 || id == 1 { // MTI and bitmap are rendered separately
			continue
		}
		filters = append(filters, iso8583.FilterField(strconv.Itoa(id), canonical))
	}

	for id := range panFieldIDs {
		filters = append(filters, iso8583.FilterField(id, mask(maskPAN)))
	}
	for id := range trackFieldIDs {
		filters = append(filters, iso8583.FilterField(id, mask(maskTrack)))
	}
	for id := range secretFieldIDs {
		filters = append(filters, iso8583.FilterField(id, mask(maskAll)))
	}
	// Sensitive EMV/TLV tags are masked by their tag key, so a tag is covered
	// wherever it nests (55.57, 127.57, 55.70.57) because describe applies the
	// same filter map to every composite level.
	for _, tag := range cardholderEMVTags {
		filters = append(filters, iso8583.FilterField(tag, mask(maskAll)))
	}
	// Content-scan the free-form / private fields so an embedded PAN does not leak
	// through the text view.
	for id := range contentScanFieldIDs {
		filters = append(filters, iso8583.FilterField(id, mask(maskEmbeddedSensitive)))
	}
	return filters
}

// cardholderEMVTags are TLV tags that carry the PAN, track, or PIN in EMV form
// (5A account number, 56/57/9F6B track data, 99 PIN, 9F1F/9F20 track
// discretionary). They are masked anywhere a message is displayed, in any TLV
// container and at any nesting depth.
var cardholderEMVTags = []string{"5A", "57", "56", "99", "9F1F", "9F20", "9F6B"}

// isSensitiveTLVTag reports whether a TLV tag carries cardholder data and must be
// masked wherever it appears.
func isSensitiveTLVTag(tag string) bool {
	for _, t := range cardholderEMVTags {
		if t == tag {
			return true
		}
	}
	return false
}

// cryptogramTag is the EMV Application Cryptogram. It is not cardholder data, so
// view keeps it for debugging, but redact masks it as well.
const cryptogramTag = "9F26"

// MaskCardholderData masks the PAN, track data, PIN, and the EMV tags that carry
// them, in place. It returns the sorted list of masked paths. This is the
// masking that `view` applies so its output never leaks cardholder data, and it
// is the base for `redact`.
func MaskCardholderData(doc *messageio.Document) []string {
	maskedSet := map[string]bool{}
	markMasked := func(path string) { maskedSet[path] = true }

	// Positional cardholder fields, in whichever representation the document
	// carries them. A binary representation (hex bytes, no clean digit boundary)
	// is fully masked; a text representation keeps the BIN and last four.
	maskPositional(doc, panFieldIDs, maskPAN, markMasked)
	maskPositional(doc, trackFieldIDs, maskTrack, markMasked)
	maskPositional(doc, secretFieldIDs, maskAll, markMasked)

	// Sensitive TLV tags, masked by their leaf tag so they are caught in any
	// container (55, 127, ...) and at any depth (55.70.57), known or unknown.
	for path, v := range doc.BinaryFields {
		if tag, ok := leafTag(path); ok && isSensitiveTLVTag(tag) {
			doc.BinaryFields[path] = maskAll(v)
			markMasked(path)
		}
	}
	for path, v := range doc.Fields {
		if tag, ok := leafTag(path); ok && isSensitiveTLVTag(tag) {
			doc.Fields[path] = maskAll(v)
			markMasked(path)
		}
	}

	// Free-form fields can embed a PAN/track that no positional or tag rule
	// covers, in text ("PAN=4111...") or in hex-encoded bytes.
	for path, v := range doc.Fields {
		if isContentScanPath(path) {
			if mv := maskEmbeddedSensitive(v); mv != v {
				doc.Fields[path] = mv
				markMasked(path)
			}
		}
	}
	for path, v := range doc.BinaryFields {
		if isContentScanPath(path) && binaryEmbedsSensitive(v) {
			doc.BinaryFields[path] = maskAll(v)
			markMasked(path)
		}
	}

	masked := make([]string, 0, len(maskedSet))
	for path := range maskedSet {
		masked = append(masked, path)
	}
	sort.Strings(masked)
	return masked
}

// maskPositional masks the given positional field ids in whichever representation
// the document uses: a text value gets textMask (which may keep the BIN), while a
// binary (hex) value is fully masked because it has no clean digit boundary.
func maskPositional(doc *messageio.Document, ids map[string]bool, textMask func(string) string, mark func(string)) {
	for id := range ids {
		if v, ok := doc.Fields[id]; ok {
			doc.Fields[id] = textMask(v)
			mark(id)
			continue
		}
		if v, ok := doc.BinaryFields[id]; ok {
			doc.BinaryFields[id] = maskAll(v)
			mark(id)
		}
	}
}

// RedactMessage unpacks a message and returns a sanitized document with
// cardholder data and secrets masked, plus the sorted list of redacted paths.
// Masking is deterministic and length-preserving. The result is meant for safe
// sharing, not for re-packing.
func RedactMessage(spec *iso8583.MessageSpec, raw []byte) (messageio.Document, []string, error) {
	doc, err := MessageToDocument(spec, raw)
	if err != nil {
		return messageio.Document{}, nil, err
	}

	masked := MaskCardholderData(&doc)
	// redact also masks the application cryptogram, in any TLV container/depth.
	for path, v := range doc.BinaryFields {
		if tag, ok := leafTag(path); ok && tag == cryptogramTag {
			doc.BinaryFields[path] = maskAll(v)
			masked = append(masked, path)
		}
	}

	// Unknown TLV tags can hold anything, including cardholder data, so a
	// "safe to share" document must mask them too.
	msg := iso8583.NewMessage(spec)
	if err := safeUnpack(msg, raw); err != nil {
		return messageio.Document{}, nil, errors.New(diagnoseUnpack(err, raw).String())
	}
	masked = append(masked, maskUnknownInDocument(&doc, collectUnknownTags(msg))...)

	sort.Strings(masked)
	return doc, masked, nil
}

// maskUnknownTagValues returns copies of the unknown tags with their raw values
// masked. Unknown tags can carry cardholder data (e.g. an unmapped Track 2
// tag), so view and validate print the tag path but never its bytes.
func maskUnknownTagValues(tags []UnknownTag) []UnknownTag {
	if len(tags) == 0 {
		return nil
	}
	masked := make([]UnknownTag, len(tags))
	for i, t := range tags {
		t.Raw = maskAll(t.Raw)
		masked[i] = t
	}
	return masked
}

// maskUnknownInDocument masks the binary-field values of the given unknown TLV
// tags in place and returns the paths it masked. view and redact call this so
// unknown tags never leak; convert skips it so they survive a round trip.
func maskUnknownInDocument(doc *messageio.Document, tags []UnknownTag) []string {
	var masked []string
	for _, t := range tags {
		if v, ok := doc.BinaryFields[t.Path]; ok {
			doc.BinaryFields[t.Path] = maskAll(v)
			masked = append(masked, t.Path)
		}
	}
	return masked
}

// maskUnknownInText masks the value of each unknown TLV tag in moov's Describe
// output. It works line by line and only touches lines that moov marks as
// "Unknown TLV tag", so a known tag is never masked just because it happens to
// share the same bytes as an unknown one.
func maskUnknownInText(body string, tags []UnknownTag) string {
	if len(tags) == 0 {
		return body
	}
	const marker = "Unknown TLV tag"
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if !strings.Contains(line, marker) {
			continue
		}
		idx := strings.LastIndex(line, ": ")
		if idx < 0 {
			continue
		}
		prefix, value := line[:idx+2], line[idx+2:]
		lines[i] = prefix + maskAll(value)
	}
	return strings.Join(lines, "\n")
}

// maskValueForDiff returns the display form of a field value for diff output.
// It applies the same masking view uses so diff is as safe to share as view:
// the PAN keeps BIN + last four, track data keeps its BIN, and PIN, cardholder
// TLV tags (any container/depth), and unknown TLV tags are fully masked.
// unknownPaths lists the TLV tags the active spec does not define.
func maskValueForDiff(path, value string, unknownPaths map[string]bool) string {
	if tag, ok := leafTag(path); ok {
		if isSensitiveTLVTag(tag) || unknownPaths[path] {
			return maskAll(value)
		}
	}
	id := topLevelID(path)
	if path == id { // a top-level positional field
		switch {
		case panFieldIDs[id]:
			return maskPAN(value)
		case trackFieldIDs[id]:
			return maskTrack(value)
		case secretFieldIDs[id]:
			return maskAll(value)
		}
	}
	if isContentScanPath(path) {
		return maskEmbeddedSensitive(value)
	}
	return value
}

// maskPAN keeps the 6-digit BIN and the last 4 digits and masks the rest.
func maskPAN(v string) string {
	if len(v) <= 10 {
		return strings.Repeat("*", len(v))
	}
	return v[:6] + strings.Repeat("*", len(v)-10) + v[len(v)-4:]
}

// maskTrack keeps the leading BIN and masks everything after it, so the PAN,
// expiry, service code, and discretionary data are all removed.
func maskTrack(v string) string {
	if len(v) <= 6 {
		return strings.Repeat("*", len(v))
	}
	return v[:6] + strings.Repeat("*", len(v)-6)
}

// maskAll masks the whole value, preserving its length.
func maskAll(v string) string {
	return strings.Repeat("*", len(v))
}

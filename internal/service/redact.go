package service

import (
	"errors"
	"sort"
	"strings"

	"github.com/moov-io/iso8583"

	"github.com/nao1215/iso8583tool/internal/messageio"
)

// cardholderEMVTags are Field 55 tags that carry the PAN, track, or PIN in EMV
// form. They are masked anywhere a message is displayed.
var cardholderEMVTags = []string{"5A", "57", "56", "99", "9F1F", "9F20"}

// cryptogramTag is the EMV Application Cryptogram. It is not cardholder data, so
// view keeps it for debugging, but redact masks it as well.
const cryptogramTag = "9F26"

// MaskCardholderData masks the PAN, track data, PIN, and the EMV tags that carry
// them, in place. It returns the sorted list of masked paths. This is the
// masking that `view` applies so its output never leaks cardholder data, and it
// is the base for `redact`.
func MaskCardholderData(doc *messageio.Document) []string {
	var masked []string
	mask := func(store map[string]string, path string, fn func(string) string) {
		v, ok := store[path]
		if !ok {
			return
		}
		store[path] = fn(v)
		masked = append(masked, path)
	}

	mask(doc.Fields, "2", maskPAN)        // Primary Account Number
	mask(doc.Fields, "20", maskPAN)       // Secondary PAN
	mask(doc.Fields, "35", maskTrack)     // Track 2
	mask(doc.Fields, "36", maskTrack)     // Track 3
	mask(doc.Fields, "45", maskTrack)     // Track 1
	mask(doc.BinaryFields, "52", maskAll) // PIN data
	for _, tag := range cardholderEMVTags {
		mask(doc.BinaryFields, "55."+tag, maskAll)
	}

	sort.Strings(masked)
	return masked
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
	// redact also masks the application cryptogram.
	if v, ok := doc.BinaryFields["55."+cryptogramTag]; ok {
		doc.BinaryFields["55."+cryptogramTag] = maskAll(v)
		masked = append(masked, "55."+cryptogramTag)
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

// maskUnknownInText replaces the raw hex of each unknown TLV tag in moov's
// Describe output with a length-preserving mask, so the text view does not leak
// bytes the document and unknown-tag list already mask.
func maskUnknownInText(body string, tags []UnknownTag) string {
	for _, t := range tags {
		if t.Raw == "" {
			continue
		}
		body = strings.ReplaceAll(body, t.Raw, maskAll(t.Raw))
	}
	return body
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

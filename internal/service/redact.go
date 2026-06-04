package service

import (
	"sort"
	"strings"

	"github.com/moov-io/iso8583"

	"github.com/nao1215/iso8583tool/internal/messageio"
)

// sensitiveEMVTags are Field 55 tags that carry cardholder data or secrets and
// must be masked before a message is shared.
var sensitiveEMVTags = map[string]bool{
	"5A":   true, // Application PAN
	"57":   true, // Track 2 equivalent data
	"56":   true, // Track 1 equivalent data
	"99":   true, // Transaction PIN data
	"9F1F": true, // Track 1 discretionary data
	"9F20": true, // Track 2 discretionary data
	"9F26": true, // Application Cryptogram
}

// RedactMessage unpacks a message and returns a sanitized document with
// cardholder data and secrets masked, plus the sorted list of redacted paths.
// Masking is deterministic and length-preserving. The result is meant for safe
// sharing (e.g. pasting into a chat), not for re-packing.
func RedactMessage(spec *iso8583.MessageSpec, raw []byte) (messageio.Document, []string, error) {
	doc, err := MessageToDocument(spec, raw)
	if err != nil {
		return messageio.Document{}, nil, err
	}

	var redacted []string
	mask := func(store map[string]string, path string, fn func(string) string) {
		v, ok := store[path]
		if !ok {
			return
		}
		store[path] = fn(v)
		redacted = append(redacted, path)
	}

	mask(doc.Fields, "2", maskPAN)        // Primary Account Number
	mask(doc.Fields, "35", maskTrack)     // Track 2 data
	mask(doc.Fields, "36", maskTrack)     // Track 3 data
	mask(doc.Fields, "45", maskTrack)     // Track 1 data
	mask(doc.BinaryFields, "52", maskAll) // PIN data

	for tag := range sensitiveEMVTags {
		mask(doc.BinaryFields, "55."+tag, maskAll)
	}

	sort.Strings(redacted)
	return doc, redacted, nil
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

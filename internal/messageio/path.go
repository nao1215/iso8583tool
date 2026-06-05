package messageio

import (
	"fmt"
	"strconv"
	"strings"
)

// maxFieldID is the highest addressable ISO 8583 data field. A primary bitmap
// plus secondary and tertiary bitmaps cover fields 2..128; 0 is the MTI and 1 is
// the primary bitmap, both of which the message owns and a document must not set.
const maxFieldID = 128

// Path is a parsed ISO 8583 field path: a top-level field id followed by any
// nested dot-separated subfield segments, for example "55", "55.9F02", or
// "55.70.9F02". It centralizes the dot-path parsing the pack, redact, view, and
// diff layers all rely on, so "what is the field id" and "what is the leaf tag"
// have a single answer.
type Path struct {
	raw      string
	segments []string
}

// NewPath parses a dot-path into its segments. It does not validate the path;
// use canonicalPath / ParseDocument for that. strings.Split always yields at
// least one segment, so TopLevelID is always defined.
func NewPath(raw string) Path {
	return Path{raw: raw, segments: strings.Split(raw, ".")}
}

// String returns the original dot-path.
func (p Path) String() string { return p.raw }

// Segments returns the dot-separated parts of the path.
func (p Path) Segments() []string { return p.segments }

// TopLevelID returns the top-level field-id segment ("55.70.9F02" -> "55").
func (p Path) TopLevelID() string { return p.segments[0] }

// IsTopLevel reports whether the path addresses a top-level field with no
// nested subfield ("55" is top-level; "55.9F02" is not).
func (p Path) IsTopLevel() bool { return len(p.segments) == 1 }

// Leaf returns the trailing segment (the TLV tag or subfield id) and true when
// the path is nested. A top-level path has no leaf tag, so it returns ("", false).
func (p Path) Leaf() (string, bool) {
	if len(p.segments) < 2 {
		return "", false
	}
	return p.segments[len(p.segments)-1], true
}

// canonicalPath validates a single document path and returns its canonical form.
//
// Canonicalization collapses spellings that address the same field — "02" and
// "2", or the BER-TLV tags "55.9f02" and "55.9F02" — to one key. Callers use the
// canonical form to detect duplicate aliases that would otherwise silently
// overwrite one another during packing.
func canonicalPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("invalid path %q: empty path", path)
	}
	if path != strings.TrimSpace(path) {
		return "", fmt.Errorf("invalid path %q: leading/trailing whitespace", path)
	}

	segments := NewPath(path).Segments()
	canon := make([]string, len(segments))
	for i, seg := range segments {
		if seg != strings.TrimSpace(seg) {
			return "", fmt.Errorf("invalid path %q: leading/trailing whitespace", path)
		}
		if seg == "" {
			return "", fmt.Errorf("invalid path %q: empty segment", path)
		}
		if i == 0 {
			id, err := canonicalTopLevelID(seg)
			if err != nil {
				return "", err
			}
			canon[i] = id
			continue
		}
		// Sub-paths are either BER-TLV tags (hex, case-insensitive) or positional
		// ids. Upper-casing leaves digits untouched and folds tag spellings so
		// "9f02" and "9F02" cannot smuggle the same tag in twice.
		canon[i] = strings.ToUpper(seg)
	}
	return strings.Join(canon, "."), nil
}

// canonicalTopLevelID validates the first path segment as an ISO 8583 field id
// and returns its canonical (leading-zero-free) form.
func canonicalTopLevelID(seg string) (string, error) {
	id, err := strconv.Atoi(seg)
	if err != nil {
		return "", fmt.Errorf("invalid field id %q: must be a number 2..%d", seg, maxFieldID)
	}
	switch {
	case id == 0:
		return "", fmt.Errorf("path %q is reserved for mti and must not be set as a field", seg)
	case id == 1:
		return "", fmt.Errorf("path %q is the bitmap and must not be set manually", seg)
	case id < 2 || id > maxFieldID:
		return "", fmt.Errorf("invalid field id %q: must be a number 2..%d", seg, maxFieldID)
	}
	return strconv.Itoa(id), nil
}

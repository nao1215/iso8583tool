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

	segments := strings.Split(path, ".")
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

package service

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"

	"github.com/nao1215/iso8583tool/internal/messageio"
)

type WriteResult struct {
	Raw        []byte
	FieldCount int
}

func WriteMessage(doc messageio.Document, spec *iso8583.MessageSpec) (WriteResult, error) {
	if err := doc.Validate(); err != nil {
		return WriteResult{}, err
	}

	msg := iso8583.NewMessage(spec)
	msg.MTI(doc.MTI)

	fieldPaths := sortedMapKeys(doc.Fields)
	for _, path := range fieldPaths {
		value := doc.Fields[path]
		if strings.Contains(path, ".") {
			if err := msg.MarshalPath(path, value); err != nil {
				return WriteResult{}, marshalPathError("set", path, err)
			}
			continue
		}
		id, err := strconv.Atoi(path)
		if err != nil {
			return WriteResult{}, fmt.Errorf("invalid field id %q", path)
		}
		if err := msg.Field(id, value); err != nil {
			return WriteResult{}, fmt.Errorf("set %s: %w", path, err)
		}
	}

	// TLV subtags such as "55.9F02" are accumulated per composite field so that
	// known and unknown tags are packed together, preserving unknown tags.
	tlvGroups := map[int]map[string][]byte{}

	binaryPaths := sortedMapKeys(doc.BinaryFields)
	for _, path := range binaryPaths {
		rawValue := strings.ReplaceAll(doc.BinaryFields[path], " ", "")
		data, err := hex.DecodeString(rawValue)
		if err != nil {
			return WriteResult{}, fmt.Errorf("decode binary field %s: %w", path, err)
		}

		if topID, tag, ok := splitTLVPath(path); ok && isTLVComposite(spec, topID) {
			if tlvGroups[topID] == nil {
				tlvGroups[topID] = map[string][]byte{}
			}
			tlvGroups[topID][tag] = data
			continue
		}

		if strings.Contains(path, ".") {
			if err := msg.MarshalPath(path, data); err != nil {
				return WriteResult{}, marshalPathError("set binary", path, err)
			}
			continue
		}

		id, err := strconv.Atoi(path)
		if err != nil {
			return WriteResult{}, fmt.Errorf("invalid binary field id %q", path)
		}
		// A text field (String/Numeric in the spec) carries human-readable data,
		// so it must be set via "fields". Accepting raw bytes through
		// "binary_fields" would inject control/non-printable bytes that corrupt
		// the summary and validation, so reject it.
		if isTextField(spec.Fields[id]) {
			return WriteResult{}, fmt.Errorf("field %d is a text field; set it via \"fields\", not \"binary_fields\"", id)
		}
		if err := msg.BinaryField(id, data); err != nil {
			return WriteResult{}, fmt.Errorf("set binary %s: %w", path, err)
		}
	}

	for _, id := range sortedIntKeys(tlvGroups) {
		blob, err := encodeTLV(tlvGroups[id])
		if err != nil {
			return WriteResult{}, fmt.Errorf("field %d: %w", id, err)
		}
		// Nested paths such as "55.70.9F02" were already set on the composite via
		// MarshalPath. Merge the flat tags into the composite's existing TLV stream
		// rather than replacing it, so a message that mixes a top-level tag
		// ("55.82") with a constructed one ("55.70.9F02") keeps both instead of
		// dropping the nested side. GetBytes returns nil when nothing was set yet,
		// so the flat-only case is unchanged.
		existing, err := msg.GetBytes(id)
		if err != nil {
			return WriteResult{}, fmt.Errorf("field %d: %w", id, err)
		}
		merged := make([]byte, 0, len(existing)+len(blob))
		merged = append(merged, existing...)
		merged = append(merged, blob...)
		if err := msg.BinaryField(id, merged); err != nil {
			return WriteResult{}, fmt.Errorf("set field %d: %w", id, err)
		}
	}

	packed, err := msg.Pack()
	if err != nil {
		return WriteResult{}, err
	}
	return WriteResult{
		Raw:        packed,
		FieldCount: topLevelFieldCount(fieldPaths, binaryPaths),
	}, nil
}

// topLevelFieldCount counts the distinct top-level ISO field ids across the
// document's text and binary paths. A TLV subtag such as "55.9F02" counts toward
// its parent field (55), and the MTI is not a data field, so the result matches
// the field_count `doctor` reports for the same message instead of inflating to
// one entry per TLV tag.
func topLevelFieldCount(fieldPaths, binaryPaths []string) int {
	ids := make(map[string]struct{}, len(fieldPaths)+len(binaryPaths))
	for _, p := range fieldPaths {
		ids[topLevelID(p)] = struct{}{}
	}
	for _, p := range binaryPaths {
		ids[topLevelID(p)] = struct{}{}
	}
	return len(ids)
}

// topLevelID returns the field id portion of a dot-path ("55.9F02" -> "55").
func topLevelID(path string) string {
	if i := strings.IndexByte(path, '.'); i >= 0 {
		return path[:i]
	}
	return path
}

// marshalPathError turns moov's internal "not a PathMarshaler" failure into a
// user-facing explanation: in the active spec the field is a plain value, so it
// has no dot-path subfields. Other errors are wrapped verbatim.
func marshalPathError(label, path string, err error) error {
	if strings.Contains(err.Error(), "not a PathMarshaler") {
		top := topLevelID(path)
		return fmt.Errorf("%s %s: field %s is a plain field in this spec and has no dot-path subfields; set field %s as a whole value instead", label, path, top, top)
	}
	return fmt.Errorf("%s %s: %w", label, path, err)
}

// isTextField reports whether the spec models a field as human-readable text: a
// String or Numeric with ASCII encoding. Such a field must be set through
// "fields"; routing raw bytes to it via "binary_fields" would inject
// control/non-printable bytes. A String/Numeric with a binary-ish encoding (for
// example the PIN field 52, a String with BytesToASCIIHex), and Binary, Hex,
// Track, and Composite fields, legitimately carry bytes via "binary_fields".
func isTextField(f field.Field) bool {
	switch f.(type) {
	case *field.String, *field.Numeric:
		return f.Spec() != nil && f.Spec().Enc == encoding.ASCII
	default:
		return false
	}
}

// splitTLVPath splits a flat TLV path such as "55.9F02" into its field id and
// tag. Deeper paths (for example "127.25.1") are not treated as TLV.
func splitTLVPath(path string) (id int, tag string, ok bool) {
	parts := strings.Split(path, ".")
	if len(parts) != 2 {
		return 0, "", false
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	return id, parts[1], true
}

// isTLVComposite reports whether the field is a BER-TLV composite (e.g. F55).
func isTLVComposite(spec *iso8583.MessageSpec, id int) bool {
	f, ok := spec.Fields[id]
	if !ok {
		return false
	}
	composite, ok := f.(*field.Composite)
	if !ok {
		return false
	}
	s := composite.Spec()
	return s.Tag != nil && s.Tag.Enc != nil
}

// encodeTLV builds a BER-TLV byte stream from tag->value entries. Tag order is
// canonicalized again by moov on Pack, so any stable order is fine here.
func encodeTLV(entries map[string][]byte) ([]byte, error) {
	tags := make([]string, 0, len(entries))
	for tag := range entries {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	var out []byte
	for _, tag := range tags {
		tagBytes, err := hex.DecodeString(tag)
		if err != nil || len(tagBytes) == 0 {
			return nil, fmt.Errorf("invalid TLV tag %q", tag)
		}
		value := entries[tag]
		out = append(out, tagBytes...)
		out = append(out, encodeBERLength(len(value))...)
		out = append(out, value...)
	}
	return out, nil
}

func encodeBERLength(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n & 0x7f)}
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte(n & 0xff)}, b...)
		n >>= 8
	}
	// b holds at most 8 length bytes, so 0x80|len(b) is always one byte.
	lengthPrefix := byte(0x80 | (len(b) & 0x7f)) //nolint:gosec // len(b) <= 8, value fits in a byte
	return append([]byte{lengthPrefix}, b...)
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedIntKeys[V any](values map[int]V) []int {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

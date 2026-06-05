package service

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moov-io/iso8583"
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
				return WriteResult{}, fmt.Errorf("set %s: %w", path, err)
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
				return WriteResult{}, fmt.Errorf("set binary %s: %w", path, err)
			}
			continue
		}

		id, err := strconv.Atoi(path)
		if err != nil {
			return WriteResult{}, fmt.Errorf("invalid binary field id %q", path)
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
		if err := msg.BinaryField(id, blob); err != nil {
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

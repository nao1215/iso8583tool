package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"

	"github.com/nao1215/iso8583tool/internal/messageio"
)

// MessageToDocument unpacks a packed message and renders it as a message
// document (the same shape `write`/convert accept), so the result round-trips.
// BER-TLV composites (e.g. Field 55) are emitted per tag and preserve unknown
// tags; positional and nested composites are expanded into their child paths.
func MessageToDocument(spec *iso8583.MessageSpec, raw []byte) (messageio.Document, error) {
	msg := iso8583.NewMessage(spec)
	if err := safeUnpack(msg, raw); err != nil {
		// Present the same field-aware, actionable diagnosis view emits, so
		// convert/diff/redact no longer leak moov's raw "failed to decode
		// content" internals for the identical "won't unpack under this spec"
		// failure. Run doctor to find the right preset.
		return messageio.Document{}, errors.New(diagnoseUnpack(err, raw).String())
	}
	mti, err := msg.GetMTI()
	if err != nil {
		return messageio.Document{}, errors.New(diagnoseUnpack(err, raw).String())
	}

	doc := messageio.Document{
		MTI:          mti,
		Fields:       map[string]string{},
		BinaryFields: map[string]string{},
	}

	fields := msg.GetFields()
	ids := make([]int, 0, len(fields))
	for id := range fields {
		if id == 0 || id == 1 { // MTI and bitmap are implicit
			continue
		}
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		if err := appendFieldToDocument(&doc, strconv.Itoa(id), fields[id]); err != nil {
			return messageio.Document{}, err
		}
	}

	// Unknown BER-TLV tags (e.g. 55.DF8129) are kept as binary so they survive a
	// round trip even though the spec does not define them.
	for tagPath, tagField := range iso8583.UnknownTags(msg) {
		if _, exists := doc.BinaryFields[tagPath]; exists {
			continue
		}
		b, err := tagField.Bytes()
		if err != nil {
			return messageio.Document{}, fmt.Errorf("field %s: %w", tagPath, err)
		}
		doc.BinaryFields[tagPath] = upperHex(b)
	}

	if len(doc.Fields) == 0 {
		doc.Fields = nil
	}
	if len(doc.BinaryFields) == 0 {
		doc.BinaryFields = nil
	}
	return doc, nil
}

// appendFieldToDocument writes a field (and any subfields) into the document at
// the given dot-path. BER-TLV composites become one binary entry per tag;
// other composites are recursed; primitives become text (or hex when they are
// not printable strings).
func appendFieldToDocument(doc *messageio.Document, path string, f field.Field) error {
	if isTLVCompositeField(f) {
		sub := f.(compositeField).GetSubfields()
		for _, tag := range sortedFieldKeys(sub) {
			child := sub[tag]
			// A constructed (nested) TLV tag is itself a composite; recurse so its
			// children get their own leaf paths (55.70.9F02) instead of collapsing
			// into the parent tag's raw blob (55.70).
			if isTLVCompositeField(child) {
				if err := appendFieldToDocument(doc, path+"."+tag, child); err != nil {
					return err
				}
				continue
			}
			b, err := child.Bytes()
			if err != nil {
				return fmt.Errorf("field %s.%s: %w", path, tag, err)
			}
			doc.BinaryFields[path+"."+tag] = upperHex(b)
		}
		return nil
	}

	if container, ok := f.(compositeField); ok {
		sub := container.GetSubfields()
		for _, name := range sortedFieldKeys(sub) {
			if err := appendFieldToDocument(doc, path+"."+name, sub[name]); err != nil {
				return err
			}
		}
		return nil
	}

	// Binary-encoded primitives (e.g. F52 PIN, MAC fields) carry raw bytes that
	// are not meaningful as text and would put control characters in the JSON.
	// Emit them as hex in binary_fields so the document is clean and, crucially,
	// so masking (redact/view) can reach them by path.
	if isBinaryEncodedField(f) {
		b, err := f.Bytes()
		if err != nil {
			return fmt.Errorf("field %s: %w", path, err)
		}
		doc.BinaryFields[path] = upperHex(b)
		return nil
	}

	str, err := f.String()
	if err != nil {
		b, bErr := f.Bytes()
		if bErr != nil {
			return fmt.Errorf("field %s: %w", path, err)
		}
		doc.BinaryFields[path] = upperHex(b)
		return nil
	}
	doc.Fields[path] = canonicalFieldValue(f, str)
	return nil
}

// isBinaryEncodedField reports whether a primitive field is encoded as raw bytes
// on the wire (binary or ASCII-hex), so its value is best represented as hex.
func isBinaryEncodedField(f field.Field) bool {
	s := f.Spec()
	if s == nil || s.Enc == nil {
		return false
	}
	switch s.Enc {
	case encoding.Binary, encoding.BytesToASCIIHex, encoding.ASCIIHexToBytes:
		return true
	default:
		return false
	}
}

type compositeField interface {
	GetSubfields() map[string]field.Field
}

// isTLVCompositeField reports whether the field is a BER-TLV composite (its
// subfields are addressed by encoded tags), as opposed to a positional one.
func isTLVCompositeField(f field.Field) bool {
	composite, ok := f.(*field.Composite)
	if !ok {
		return false
	}
	s := composite.Spec()
	return s != nil && s.Tag != nil && s.Tag.Enc != nil
}

func sortedFieldKeys(m map[string]field.Field) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func upperHex(b []byte) string {
	return strings.ToUpper(hex.EncodeToString(b))
}

// canonicalFieldValue restores the on-the-wire, edit-ready representation of a
// primitive field value. Fixed-length fields that declare a padder (e.g. the
// zero-padded amount fields F3/F4) lose their padding when unpacked, so
// field.String() yields "5000" instead of "000000005000". Re-applying the
// field's own padder reproduces the canonical value that the JSON samples and
// the README document, which also packs back to the identical bytes. Fields
// without a padder, and variable-length fields, are returned unchanged.
func canonicalFieldValue(f field.Field, str string) string {
	spec := f.Spec()
	if spec == nil || spec.Pad == nil || spec.Pref == nil {
		return str
	}
	if !strings.HasSuffix(spec.Pref.Inspect(), ".Fixed") {
		return str
	}
	return string(spec.Pad.Pad([]byte(str), spec.Length))
}

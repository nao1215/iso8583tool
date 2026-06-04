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

// MessageToDocument unpacks a packed message and renders it as a message
// document (the same shape `write`/convert accept), so the result round-trips.
// Composite fields (e.g. Field 55) are emitted as one raw-hex binary field to
// preserve unknown TLV tags.
func MessageToDocument(spec *iso8583.MessageSpec, raw []byte) (messageio.Document, error) {
	msg := iso8583.NewMessage(spec)
	if err := safeUnpack(msg, raw); err != nil {
		return messageio.Document{}, err
	}
	mti, err := msg.GetMTI()
	if err != nil {
		return messageio.Document{}, err
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

	unknown := iso8583.UnknownTags(msg)

	for _, id := range ids {
		path := strconv.Itoa(id)
		f := fields[id]

		// Composite (TLV) fields like F55 are emitted per tag so individual EMV
		// tags can be edited; unknown tags are included so they round-trip.
		if container, ok := f.(interface {
			GetSubfields() map[string]field.Field
		}); ok {
			for tag, sub := range container.GetSubfields() {
				b, err := sub.Bytes()
				if err != nil {
					return messageio.Document{}, fmt.Errorf("field %s.%s: %w", path, tag, err)
				}
				doc.BinaryFields[path+"."+tag] = strings.ToUpper(hex.EncodeToString(b))
			}
			prefix := path + "."
			for tagPath, tagField := range unknown {
				if !strings.HasPrefix(tagPath, prefix) {
					continue
				}
				if _, exists := doc.BinaryFields[tagPath]; exists {
					continue
				}
				b, err := tagField.Bytes()
				if err != nil {
					return messageio.Document{}, fmt.Errorf("field %s: %w", tagPath, err)
				}
				doc.BinaryFields[tagPath] = strings.ToUpper(hex.EncodeToString(b))
			}
			continue
		}

		str, err := f.String()
		if err != nil {
			b, bErr := f.Bytes()
			if bErr != nil {
				return messageio.Document{}, fmt.Errorf("field %d: %w", id, err)
			}
			doc.BinaryFields[path] = strings.ToUpper(hex.EncodeToString(b))
			continue
		}
		doc.Fields[path] = canonicalFieldValue(f, str)
	}

	if len(doc.Fields) == 0 {
		doc.Fields = nil
	}
	if len(doc.BinaryFields) == 0 {
		doc.BinaryFields = nil
	}
	return doc, nil
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

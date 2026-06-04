package service

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moov-io/iso8583"

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

	binaryPaths := sortedMapKeys(doc.BinaryFields)
	for _, path := range binaryPaths {
		rawValue := strings.ReplaceAll(doc.BinaryFields[path], " ", "")
		data, err := hex.DecodeString(rawValue)
		if err != nil {
			return WriteResult{}, fmt.Errorf("decode binary field %s: %w", path, err)
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

	packed, err := msg.Pack()
	if err != nil {
		return WriteResult{}, err
	}
	return WriteResult{
		Raw:        packed,
		FieldCount: len(fieldPaths) + len(binaryPaths) + 1,
	}, nil
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

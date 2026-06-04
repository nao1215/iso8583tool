package service

import (
	"errors"
	"fmt"
	"strings"

	iso8583errors "github.com/moov-io/iso8583/errors"
)

// UnpackDiagnosis describes where and why a message failed to unpack.
type UnpackDiagnosis struct {
	Path  string // dot-path of the field that failed, e.g. "1" or "55.9F26"
	Cause string // root cause message
	Bytes int    // number of input bytes
}

// diagnoseUnpack turns a raw unpack error into a field-aware diagnosis so the
// output can point at the part of the message that is broken.
func diagnoseUnpack(err error, raw []byte) UnpackDiagnosis {
	d := UnpackDiagnosis{Cause: rootCause(err).Error(), Bytes: len(raw)}

	var unpackErr *iso8583errors.UnpackError
	if errors.As(err, &unpackErr) {
		ids := make([]string, 0)
		for _, id := range unpackErr.FieldIDs() {
			if strings.TrimSpace(id) != "" {
				ids = append(ids, id)
			}
		}
		d.Path = strings.Join(ids, ".")
	}
	return d
}

// String renders a single-line, human-readable explanation.
func (d UnpackDiagnosis) String() string {
	if d.Path != "" {
		return fmt.Sprintf("cannot unpack field %s: %s (input was %d bytes)", d.Path, d.Cause, d.Bytes)
	}
	return fmt.Sprintf("cannot unpack message: %s (input was %d bytes)", d.Cause, d.Bytes)
}

func rootCause(err error) error {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
}

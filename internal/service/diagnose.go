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

// Malformed reports whether the failure means the message is truncated or
// corrupt rather than merely decoded under the wrong spec. It is conservative:
// it fires only when the message is too short to hold even its MTI or bitmap (no
// spec can rescue that) or when the library panicked on an overrun. A failure at
// a data field is left to the wrong-spec path, which points at doctor.
func (d UnpackDiagnosis) Malformed() bool {
	switch d.Path {
	case "", "0", "1": // no field id, the MTI, or the bitmap
		return true
	}
	return strings.Contains(strings.ToLower(d.Cause), "malformed")
}

// looksTruncated reports whether an unpack cause reads like the message is
// truncated or malformed rather than merely decoded under the wrong spec.
func looksTruncated(cause string) bool {
	c := strings.ToLower(cause)
	for _, marker := range []string{
		"not enough data",
		"out of range",
		"slice bounds",
		"unexpected eof",
		"malformed",
		"overrun",
	} {
		if strings.Contains(c, marker) {
			return true
		}
	}
	return false
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

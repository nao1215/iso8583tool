package service

import (
	"fmt"

	"github.com/moov-io/iso8583"
)

// safeUnpack unpacks a message but converts a panic from the underlying library
// into an error. Adversarial input (e.g. a BER-TLV length that overruns the
// buffer) can make moov-io/iso8583 panic with a slice-bounds error; for a
// debugging CLI that handles untrusted captures, that must surface as a normal
// "malformed message" error, never a crash.
func safeUnpack(msg *iso8583.Message, raw []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("malformed message: %v", r)
		}
	}()
	return msg.Unpack(raw)
}

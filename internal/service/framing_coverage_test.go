package service

import (
	"bytes"
	"errors"
	"testing"
)

// errReader returns a non-EOF error on Read, to exercise readBody's generic
// error branch.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestFramingEncode(t *testing.T) {
	t.Parallel()

	// 2-byte binary: a 3-byte payload gets a big-endian length header.
	out, err := Framing2ByteBinary.Encode([]byte{1, 2, 3})
	if err != nil || !bytes.Equal(out, []byte{0x00, 0x03, 1, 2, 3}) {
		t.Errorf("2byte encode = %v,%v", out, err)
	}
	// 2-byte binary: an oversized payload is rejected.
	if _, err := Framing2ByteBinary.Encode(make([]byte, 0x10000)); err == nil {
		t.Error("2byte encode of an oversized payload should error")
	}

	// 4-digit ASCII: header is the zero-padded decimal length.
	out, err = Framing4DigitASCII.Encode([]byte("hi"))
	if err != nil || string(out) != "0002hi" {
		t.Errorf("4digit encode = %q,%v", out, err)
	}
	if _, err := Framing4DigitASCII.Encode(make([]byte, 10000)); err == nil {
		t.Error("4digit encode of an oversized payload should error")
	}

	// None: payload passes through unchanged.
	out, err = FramingNone.Encode([]byte("raw"))
	if err != nil || string(out) != "raw" {
		t.Errorf("none encode = %q,%v", out, err)
	}

	// An invalid framing value is rejected.
	if _, err := Framing("weird").Encode([]byte("x")); err == nil {
		t.Error("invalid framing encode should error")
	}
}

func TestFramingReadResponse(t *testing.T) {
	t.Parallel()

	// 2-byte binary: strip the header and return the body.
	body, err := Framing2ByteBinary.ReadResponse(bytes.NewReader([]byte{0x00, 0x02, 'o', 'k'}))
	if err != nil || string(body) != "ok" {
		t.Errorf("2byte read = %q,%v", body, err)
	}
	// 2-byte binary: header present but body short -> error.
	if _, err := Framing2ByteBinary.ReadResponse(bytes.NewReader([]byte{0x00, 0x05, 'o'})); err == nil {
		t.Error("2byte read with a short body should error")
	}
	// 2-byte binary: header truncated -> headerReadError.
	if _, err := Framing2ByteBinary.ReadResponse(bytes.NewReader([]byte{0x00})); err == nil {
		t.Error("2byte read with a truncated header should error")
	}

	// 4-digit ASCII: read the declared body.
	body, err = Framing4DigitASCII.ReadResponse(bytes.NewReader([]byte("0003abc")))
	if err != nil || string(body) != "abc" {
		t.Errorf("4digit read = %q,%v", body, err)
	}
	// 4-digit ASCII: a non-numeric header is rejected.
	if _, err := Framing4DigitASCII.ReadResponse(bytes.NewReader([]byte("XXXXpayload"))); err == nil {
		t.Error("4digit read with a non-numeric header should error")
	}

	// None: read until EOF.
	body, err = FramingNone.ReadResponse(bytes.NewReader([]byte("streamed")))
	if err != nil || string(body) != "streamed" {
		t.Errorf("none read = %q,%v", body, err)
	}

	// Invalid framing value.
	if _, err := Framing("weird").ReadResponse(bytes.NewReader(nil)); err == nil {
		t.Error("invalid framing read should error")
	}
}

// TestReadBodyGenericError covers readBody's non-EOF error branch.
func TestReadBodyGenericError(t *testing.T) {
	t.Parallel()
	if _, err := readBody(errReader{}, 4); err == nil {
		t.Error("readBody with a failing reader should error")
	}
}

package service

import (
	"io"
	"testing"
)

// timeoutError implements net.Error with a firing timeout.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// scriptedReader returns pre-set (data, err) pairs on successive Read calls.
type scriptedReader struct {
	steps []step
	i     int
}

type step struct {
	data []byte
	err  error
}

func (s *scriptedReader) Read(p []byte) (int, error) {
	if s.i >= len(s.steps) {
		return 0, io.EOF
	}
	st := s.steps[s.i]
	s.i++
	n := copy(p, st.data)
	return n, st.err
}

// TestReadUntilEOFTimeoutWithBytes: a deadline that fires after some bytes have
// arrived returns those bytes (a no-framing peer that simply stopped writing).
func TestReadUntilEOFTimeoutWithBytes(t *testing.T) {
	t.Parallel()
	r := &scriptedReader{steps: []step{
		{data: []byte("partial"), err: nil},
		{data: nil, err: timeoutError{}},
	}}
	got, err := readUntilEOF(r)
	if err != nil {
		t.Fatalf("readUntilEOF: %v", err)
	}
	if string(got) != "partial" {
		t.Errorf("got %q, want partial", got)
	}
}

// TestReadUntilEOFTimeoutNoBytes: a deadline with no bytes read yet surfaces as
// an error so a stalled connection reads as a timeout.
func TestReadUntilEOFTimeoutNoBytes(t *testing.T) {
	t.Parallel()
	r := &scriptedReader{steps: []step{{data: nil, err: timeoutError{}}}}
	if _, err := readUntilEOF(r); err == nil {
		t.Error("readUntilEOF with an immediate timeout and no bytes should error")
	}
}

// TestReadUntilEOFGenericError: a non-EOF, non-timeout error is returned along
// with whatever bytes had arrived.
func TestReadUntilEOFGenericError(t *testing.T) {
	t.Parallel()
	r := &scriptedReader{steps: []step{
		{data: []byte("some"), err: nil},
		{data: nil, err: io.ErrClosedPipe},
	}}
	got, err := readUntilEOF(r)
	if err == nil {
		t.Error("readUntilEOF should surface a generic read error")
	}
	if string(got) != "some" {
		t.Errorf("got %q, want the bytes read so far", got)
	}
}

// TestReadUntilEOFLimit: an endless stream is capped at maxUnframedResponseBytes.
func TestReadUntilEOFLimit(t *testing.T) {
	t.Parallel()
	chunk := make([]byte, 4096)
	steps := make([]step, 0, 300)
	for i := 0; i < 300; i++ {
		steps = append(steps, step{data: chunk, err: nil})
	}
	r := &scriptedReader{steps: steps}
	if _, err := readUntilEOF(r); err == nil {
		t.Error("readUntilEOF should reject a stream over the byte limit")
	}
}

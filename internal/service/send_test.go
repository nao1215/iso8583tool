package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestParseFraming(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want Framing
		ok   bool
	}{
		{"2byte-binary", Framing2ByteBinary, true},
		{"4digit-ascii", Framing4DigitASCII, true},
		{"none", FramingNone, true},
		{" none ", FramingNone, true},
		{"bogus", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, err := ParseFraming(tc.in)
		if tc.ok {
			if err != nil {
				t.Errorf("ParseFraming(%q) returned error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseFraming(%q) = %q, want %q", tc.in, got, tc.want)
			}
			continue
		}
		if err == nil {
			t.Errorf("ParseFraming(%q) expected an error", tc.in)
		}
	}
}

func TestFramingEncodeAndReadResponseRoundTrip(t *testing.T) {
	t.Parallel()
	payload := []byte{0x08, 0x10, 0x00, 0x01, 0x02}
	for _, framing := range []Framing{Framing2ByteBinary, Framing4DigitASCII, FramingNone} {
		t.Run(string(framing), func(t *testing.T) {
			t.Parallel()
			framed, err := framing.Encode(payload)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			switch framing {
			case Framing2ByteBinary:
				if framed[0] != 0x00 || framed[1] != 0x05 {
					t.Errorf("2byte header = %x, want 0005", framed[:2])
				}
			case Framing4DigitASCII:
				if got := string(framed[:4]); got != "0005" {
					t.Errorf("4digit header = %q, want 0005", got)
				}
			case FramingNone:
				if !bytes.Equal(framed, payload) {
					t.Errorf("none framing changed the payload: %x", framed)
				}
			}
			got, err := framing.ReadResponse(bytes.NewReader(framed))
			if err != nil {
				t.Fatalf("ReadResponse: %v", err)
			}
			if !bytes.Equal(got, payload) {
				t.Errorf("round-trip payload = %x, want %x", got, payload)
			}
		})
	}
}

func TestFramingEncodeRejectsOversizedPayload(t *testing.T) {
	t.Parallel()
	if _, err := Framing2ByteBinary.Encode(make([]byte, 0x10000)); err == nil {
		t.Error("expected 2byte-binary to reject a 65536-byte payload")
	}
	if _, err := Framing4DigitASCII.Encode(make([]byte, 10000)); err == nil {
		t.Error("expected 4digit-ascii to reject a 10000-byte payload")
	}
}

func TestFramingReadResponseShortBody(t *testing.T) {
	t.Parallel()
	// Header claims 5 bytes, only 2 follow: a clear "closed early" error.
	short := []byte{0x00, 0x05, 0xAA, 0xBB}
	if _, err := Framing2ByteBinary.ReadResponse(bytes.NewReader(short)); err == nil {
		t.Fatal("expected a short-body error")
	}
}

func TestFramingReadResponseEmptyHeader(t *testing.T) {
	t.Parallel()
	if _, err := Framing2ByteBinary.ReadResponse(bytes.NewReader(nil)); err == nil {
		t.Fatal("expected an error when no header bytes are available")
	}
	if _, err := Framing4DigitASCII.ReadResponse(bytes.NewReader([]byte("12"))); err == nil {
		t.Fatal("expected an error when the 4-digit header is truncated")
	}
}

func TestFraming4DigitRejectsNonNumericHeader(t *testing.T) {
	t.Parallel()
	if _, err := Framing4DigitASCII.ReadResponse(bytes.NewReader([]byte("AB12payload"))); err == nil {
		t.Fatal("expected a non-numeric length header to be rejected")
	}
}

func TestFramingNoneReadsUntilEOF(t *testing.T) {
	t.Parallel()
	payload := []byte("0810 response bytes")
	got, err := FramingNone.ReadResponse(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("none EOF read = %q, want %q", got, payload)
	}
}

func TestFramingNoneReturnsBytesReadBeforeTimeout(t *testing.T) {
	t.Parallel()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	go func() {
		_, _ = server.Write([]byte("partial"))
		// Never close: the client's read deadline must fire.
	}()

	_ = client.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	got, err := FramingNone.ReadResponse(client)
	if err != nil {
		t.Fatalf("expected the bytes received before the deadline, got error: %v", err)
	}
	if !bytes.Equal(got, []byte("partial")) {
		t.Errorf("got %q, want %q", got, "partial")
	}
}

func TestFramingNoneTimesOutWithNoBytes(t *testing.T) {
	t.Parallel()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	_ = client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err := FramingNone.ReadResponse(client)
	if err == nil {
		t.Fatal("expected a timeout error when no bytes arrive")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("expected a timeout net.Error, got %v", err)
	}
}

// chunkWriter writes at most one byte per call so writeAll has to loop to send
// a whole frame, modelling a net.Conn that returns short writes.
type chunkWriter struct{ buf bytes.Buffer }

func (w *chunkWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return w.buf.Write(p[:1])
}

// stallWriter always reports a zero-length write with no error, the pathological
// case writeAll must not spin on forever.
type stallWriter struct{}

func (stallWriter) Write(p []byte) (int, error) { return 0, nil }

// errWriter fails after writing the first byte.
type errWriter struct{ wrote bool }

func (w *errWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.wrote = true
		return 1, nil
	}
	return 0, errors.New("boom")
}

func TestWriteAllSendsEveryByteAcrossShortWrites(t *testing.T) {
	t.Parallel()
	w := &chunkWriter{}
	payload := []byte("0800 framed request payload")
	if err := writeAll(w, payload); err != nil {
		t.Fatalf("writeAll: %v", err)
	}
	if !bytes.Equal(w.buf.Bytes(), payload) {
		t.Errorf("writeAll sent %q, want %q", w.buf.Bytes(), payload)
	}
}

func TestWriteAllStopsOnZeroProgress(t *testing.T) {
	t.Parallel()
	if err := writeAll(stallWriter{}, []byte("data")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("expected io.ErrShortWrite on a stalled writer, got %v", err)
	}
}

func TestWriteAllPropagatesError(t *testing.T) {
	t.Parallel()
	err := writeAll(&errWriter{}, []byte("data"))
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected the underlying error, got %v", err)
	}
}

func TestSendMessageConnectErrorIsWrapped(t *testing.T) {
	t.Parallel()
	// Port 0 is not connectable; the error must name what failed and the address.
	_, err := SendMessage(SendRequest{
		Address: "127.0.0.1:0",
		Payload: []byte{0x00},
		Framing: Framing2ByteBinary,
		Timeout: 200 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected a connect error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("connect to 127.0.0.1:0")) {
		t.Errorf("error %q does not explain the connect failure", err)
	}
}

func TestSendMessageRoundTrip(t *testing.T) {
	t.Parallel()
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	reply := []byte{0x08, 0x10, 0x00}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		if _, err := Framing2ByteBinary.ReadResponse(conn); err != nil {
			return
		}
		framed, _ := Framing2ByteBinary.Encode(reply)
		_, _ = conn.Write(framed)
	}()

	req := []byte{0x08, 0x00, 0x01, 0x02}
	result, err := SendMessage(SendRequest{
		Address: ln.Addr().String(),
		Payload: req,
		Framing: Framing2ByteBinary,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if !bytes.Equal(result.Response, reply) {
		t.Errorf("response = %x, want %x", result.Response, reply)
	}
	if result.SentBytes != len(req)+2 {
		t.Errorf("SentBytes = %d, want %d", result.SentBytes, len(req)+2)
	}
	if result.ReceivedBytes != len(reply)+2 {
		t.Errorf("ReceivedBytes = %d, want %d", result.ReceivedBytes, len(reply)+2)
	}
	<-done
}

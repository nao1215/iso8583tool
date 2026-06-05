package service

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/moov-io/iso8583"
)

// Framing is the message-length convention used on the wire. A single ISO 8583
// exchange frames the request and reads the response with the same convention,
// so send uses one Framing for both directions.
type Framing string

const (
	// Framing2ByteBinary prefixes the payload with a 2-byte big-endian length.
	Framing2ByteBinary Framing = "2byte-binary"
	// Framing4DigitASCII prefixes the payload with a 4-digit ASCII length header.
	Framing4DigitASCII Framing = "4digit-ascii"
	// FramingNone sends the payload with no length header; the response is read
	// until EOF (the peer closes its write side) or the deadline is reached.
	FramingNone Framing = "none"
)

// ParseFraming validates a --framing value and returns the Framing.
func ParseFraming(value string) (Framing, error) {
	switch Framing(strings.TrimSpace(value)) {
	case Framing2ByteBinary:
		return Framing2ByteBinary, nil
	case Framing4DigitASCII:
		return Framing4DigitASCII, nil
	case FramingNone:
		return FramingNone, nil
	default:
		return "", fmt.Errorf("invalid --framing %q (want 2byte-binary, 4digit-ascii, or none)", value)
	}
}

// headerLen reports how many bytes the length header occupies on the wire.
func (f Framing) headerLen() int {
	switch f {
	case Framing2ByteBinary:
		return 2
	case Framing4DigitASCII:
		return 4
	default:
		return 0
	}
}

// Encode prepends the length header for the framing to payload and returns the
// bytes to write on the wire.
func (f Framing) Encode(payload []byte) ([]byte, error) {
	switch f {
	case Framing2ByteBinary:
		if len(payload) > 0xFFFF {
			return nil, fmt.Errorf("message of %d bytes is too large for 2byte-binary framing (max 65535)", len(payload))
		}
		header := make([]byte, 2)
		binary.BigEndian.PutUint16(header, uint16(len(payload))) //nolint:gosec // length is bounded to <= 0xFFFF above
		return append(header, payload...), nil
	case Framing4DigitASCII:
		if len(payload) > 9999 {
			return nil, fmt.Errorf("message of %d bytes is too large for 4digit-ascii framing (max 9999)", len(payload))
		}
		out := make([]byte, 0, len(payload)+4)
		out = append(out, []byte(fmt.Sprintf("%04d", len(payload)))...)
		out = append(out, payload...)
		return out, nil
	case FramingNone:
		return payload, nil
	default:
		return nil, fmt.Errorf("invalid framing %q", f)
	}
}

// ReadResponse reads one framed response message from r and returns the payload
// with its length header stripped. For FramingNone it reads until EOF (or, when
// some bytes have arrived before the deadline, returns what it has).
func (f Framing) ReadResponse(r io.Reader) ([]byte, error) {
	switch f {
	case Framing2ByteBinary:
		header := make([]byte, 2)
		if _, err := io.ReadFull(r, header); err != nil {
			return nil, headerReadError(err)
		}
		return readBody(r, int(binary.BigEndian.Uint16(header)))
	case Framing4DigitASCII:
		header := make([]byte, 4)
		if _, err := io.ReadFull(r, header); err != nil {
			return nil, headerReadError(err)
		}
		n, err := strconv.Atoi(string(header))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid 4digit-ascii length header %q", string(header))
		}
		return readBody(r, n)
	case FramingNone:
		return readUntilEOF(r)
	default:
		return nil, fmt.Errorf("invalid framing %q", f)
	}
}

// readBody reads exactly n payload bytes after a length header was parsed.
func readBody(r io.Reader, n int) ([]byte, error) {
	if n == 0 {
		return []byte{}, nil
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("response declared %d payload bytes but the connection closed early", n)
		}
		return nil, err
	}
	return body, nil
}

// headerReadError turns an early close while reading a length header into a
// clear message instead of a bare EOF.
func headerReadError(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return errors.New("connection closed before a length header arrived")
	}
	return err
}

// maxUnframedResponseBytes caps a none-framing response. Without a length header
// the read runs until EOF, so a misbehaving or hostile peer could otherwise grow
// the buffer without bound; 1 MiB matches messageio's input ceiling and is far
// larger than any real ISO 8583 message.
const maxUnframedResponseBytes = 1 << 20 // 1 MiB

// readUntilEOF reads from r until EOF. If a read deadline fires after some bytes
// have already arrived, the bytes read so far are returned (a no-framing peer
// may simply stop writing); a deadline with no bytes is reported as an error so
// a stalled connection surfaces as a timeout. The total is capped at
// maxUnframedResponseBytes so an endless stream cannot exhaust memory.
func readUntilEOF(r io.Reader) ([]byte, error) {
	buf := make([]byte, 0, 512)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			if len(buf)+n > maxUnframedResponseBytes {
				return nil, fmt.Errorf("none-framing response exceeded the %d-byte limit", maxUnframedResponseBytes)
			}
			buf = append(buf, tmp[:n]...)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return buf, nil
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			if len(buf) > 0 {
				return buf, nil
			}
			return nil, err
		}
		return buf, err
	}
}

// writeAll writes every byte of data, looping over short writes. A net.Conn
// Write may return before the whole frame is on the wire; treating the first
// partial write as success would silently ship a truncated ISO 8583 message, so
// the loop continues until all bytes are written, an error occurs, or the writer
// stops making progress.
func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

// SendRequest describes a single ISO 8583 request/response exchange over TCP.
type SendRequest struct {
	Address string
	Payload []byte
	Framing Framing
	Timeout time.Duration
}

// SendResult is the outcome of a single exchange. Response holds the decoded
// payload (length header stripped); SentBytes and ReceivedBytes count the bytes
// on the wire, including the framing header.
type SendResult struct {
	RemoteAddr    string
	Response      []byte
	SentBytes     int
	ReceivedBytes int
	RTT           time.Duration
}

// SendMessage opens a single TCP connection, writes one framed request, reads
// one framed response, and returns the timing and byte counts. Each failure
// (connect, send, receive) is wrapped so the caller can report what failed.
func SendMessage(req SendRequest) (SendResult, error) {
	framed, err := req.Framing.Encode(req.Payload)
	if err != nil {
		return SendResult{}, err
	}

	dialer := net.Dialer{Timeout: req.Timeout}
	conn, err := dialer.Dial("tcp", req.Address)
	if err != nil {
		return SendResult{}, fmt.Errorf("connect to %s: %w", req.Address, err)
	}
	defer func() { _ = conn.Close() }()

	if req.Timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(req.Timeout)); err != nil {
			return SendResult{}, fmt.Errorf("set deadline on %s: %w", req.Address, err)
		}
	}

	start := time.Now()
	if err := writeAll(conn, framed); err != nil {
		return SendResult{}, fmt.Errorf("send request to %s: %w", req.Address, err)
	}

	// With no length header the peer cannot know the request ended until the
	// write side closes, so half-close after writing to let it reply and EOF.
	if req.Framing == FramingNone {
		if cw, ok := conn.(interface{ CloseWrite() error }); ok {
			if err := cw.CloseWrite(); err != nil {
				return SendResult{}, fmt.Errorf("close write side to %s: %w", req.Address, err)
			}
		}
	}

	resp, err := req.Framing.ReadResponse(conn)
	rtt := time.Since(start)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return SendResult{}, fmt.Errorf("timed out after %s waiting for a response from %s", req.Timeout, req.Address)
		}
		return SendResult{}, fmt.Errorf("receive response from %s: %w", req.Address, err)
	}

	return SendResult{
		RemoteAddr:    conn.RemoteAddr().String(),
		Response:      resp,
		SentBytes:     len(framed),
		ReceivedBytes: len(resp) + req.Framing.headerLen(),
		RTT:           rtt,
	}, nil
}

// FieldExpectation is one `--expect-field PATH=VALUE` assertion: the field at
// Path must decode to Value.
type FieldExpectation struct {
	Path  string
	Value string
}

// ExpectationFailure records one assertion that did not hold. Present is false
// when the expected field was absent from the response entirely (as opposed to
// present with a different value).
type ExpectationFailure struct {
	Label    string // human label: "MTI" or "F<path>"
	Expected string
	Actual   string
	Present  bool
}

// String renders a deterministic, single-line description of the failure.
func (f ExpectationFailure) String() string {
	if !f.Present {
		return fmt.Sprintf("%s: expected %q but the field is not present in the response", f.Label, f.Expected)
	}
	return fmt.Sprintf("%s: expected %q, got %q", f.Label, f.Expected, f.Actual)
}

// CheckExpectations compares the decoded response against the expected MTI and
// field values and returns the assertions that failed (empty when all hold).
//
// The comparison uses the unmasked, canonical field values from
// MessageToDocument — the same edit-ready representation `convert` emits — not
// the masked display strings, so an assertion on a PAN or other sensitive field
// matches its real value even though the printed view masks it. An empty
// expectMTI skips the MTI check; an empty expectFields skips field checks.
func CheckExpectations(spec *iso8583.MessageSpec, response []byte, expectMTI string, expectFields []FieldExpectation) ([]ExpectationFailure, error) {
	doc, err := MessageToDocument(spec, response)
	if err != nil {
		return nil, err
	}
	flat := FlattenDocument(doc)

	var failures []ExpectationFailure
	if want := strings.TrimSpace(expectMTI); want != "" {
		if doc.MTI != want {
			failures = append(failures, ExpectationFailure{
				Label:    "MTI",
				Expected: want,
				Actual:   doc.MTI,
				Present:  true,
			})
		}
	}

	for _, exp := range expectFields {
		path := strings.TrimSpace(exp.Path)
		actual, present := lookupFlatValue(flat, doc.MTI, path)
		if !present || actual != exp.Value {
			failures = append(failures, ExpectationFailure{
				Label:    "F" + path,
				Expected: exp.Value,
				Actual:   actual,
				Present:  present,
			})
		}
	}
	return failures, nil
}

// lookupFlatValue resolves a field path against the flattened document. The
// pseudo-paths "0" and "mti" select the MTI; EMV hex tags match
// case-insensitively (so "55.9f02" finds "55.9F02").
func lookupFlatValue(flat map[string]string, mti, path string) (string, bool) {
	if u := strings.ToUpper(path); u == "MTI" || u == "0" {
		return mti, mti != ""
	}
	if v, ok := flat[path]; ok {
		return v, true
	}
	for k, v := range flat {
		if strings.EqualFold(k, path) {
			return v, true
		}
	}
	return "", false
}

package cmd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/service"
)

// startEchoServer accepts one connection, reads a framed request, and replies
// with replyHex framed the same way. It returns the listen address.
func startEchoServer(t *testing.T, framing service.Framing, replyPayload []byte) string {
	t.Helper()
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		if _, err := framing.ReadResponse(conn); err != nil {
			return
		}
		framed, err := framing.Encode(replyPayload)
		if err != nil {
			return
		}
		// Loop over short writes so the full frame always reaches the client.
		for len(framed) > 0 {
			n, err := conn.Write(framed)
			if err != nil || n == 0 {
				return
			}
			framed = framed[n:]
		}
	}()
	return ln.Addr().String()
}

// sampleHex packs a built-in sample to its raw bytes for use as a fixture.
func sampleResponseBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(example("0810-network-echo-response.hex"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	decoded, _, err := resolveInput(raw, "auto")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return decoded
}

func TestSendJSONShape(t *testing.T) {
	t.Parallel()
	reply := sampleResponseBytes(t)
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, out, errOut := runApp("", "send", addr, example("0800-network-echo.hex"), "--format", "json")
	if code != 0 {
		t.Fatalf("send: code=%d err=%q", code, errOut)
	}

	var payload struct {
		RemoteAddr    string          `json:"remote_addr"`
		Framing       string          `json:"framing"`
		Timeout       string          `json:"timeout"`
		RTTms         float64         `json:"rtt_ms"`
		SentBytes     int             `json:"sent_bytes"`
		ReceivedBytes int             `json:"received_bytes"`
		Request       json.RawMessage `json:"request"`
		Response      json.RawMessage `json:"response"`
		RequestView   struct {
			MTI string `json:"mti"`
		} `json:"request_view"`
		ResponseView struct {
			MTI string `json:"mti"`
		} `json:"response_view"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal send json: %v\n%s", err, out)
	}
	if payload.Framing != "2byte-binary" {
		t.Errorf("framing = %q", payload.Framing)
	}
	if payload.Timeout != "5s" {
		t.Errorf("timeout = %q, want 5s", payload.Timeout)
	}
	if payload.RemoteAddr == "" {
		t.Error("remote_addr is empty")
	}
	if payload.SentBytes <= 2 || payload.ReceivedBytes <= 2 {
		t.Errorf("byte counts look wrong: sent=%d received=%d", payload.SentBytes, payload.ReceivedBytes)
	}
	if payload.RequestView.MTI != "0800" {
		t.Errorf("request_view.mti = %q, want 0800", payload.RequestView.MTI)
	}
	if payload.ResponseView.MTI != "0810" {
		t.Errorf("response_view.mti = %q, want 0810", payload.ResponseView.MTI)
	}
	if len(payload.Request) == 0 || len(payload.Response) == 0 {
		t.Error("request/response objects are missing")
	}
}

func TestSendDescribeAndStdin(t *testing.T) {
	t.Parallel()
	reply := sampleResponseBytes(t)
	addr := startEchoServer(t, service.Framing4DigitASCII, reply)

	// Read the request from stdin via "-" and use the 4digit-ascii framing.
	hexInput, err := os.ReadFile(example("0800-network-echo.hex"))
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	code, out, errOut := runApp(string(hexInput), "send", addr, "-", "--framing", "4digit-ascii")
	if code != 0 {
		t.Fatalf("send: code=%d err=%q", code, errOut)
	}
	for _, want := range []string{"Sent to:", "Framing:", "4digit-ascii", "RTT:", "Request:", "Response:", "0810"} {
		if !strings.Contains(out, want) {
			t.Errorf("describe output missing %q\n%s", want, out)
		}
	}
}

func TestSendMasksByDefault(t *testing.T) {
	t.Parallel()
	// Reply carries a real PAN; the default (masked) output must not leak it.
	raw, err := os.ReadFile(example("0110-auth-response.hex"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	reply, _, err := resolveInput(raw, "auto")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, out, errOut := runApp("", "send", addr, example("0100-auth-request.hex"), "--format", "json")
	if code != 0 {
		t.Fatalf("send: code=%d err=%q", code, errOut)
	}
	if strings.Contains(out, "4111111111111111") {
		t.Errorf("default output leaked an unmasked PAN\n%s", out)
	}
	if !strings.Contains(out, "411111******1111") {
		t.Errorf("expected a masked PAN in the response view\n%s", out)
	}
}

// panFixtureReply decodes the auth-response fixture (which carries a real PAN)
// for use as a server reply, returning both the bytes and the uppercase hex the
// JSON encoder would emit for that wire payload.
func panFixtureReply(t *testing.T) (reply []byte, panHex string) {
	t.Helper()
	raw, err := os.ReadFile(example("0110-auth-response.hex"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	reply, _, err = resolveInput(raw, "auto")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return reply, strings.ToUpper(hex.EncodeToString([]byte("4111111111111111")))
}

func TestSendJSONMasksWirePayloadByDefault(t *testing.T) {
	t.Parallel()
	reply, panHex := panFixtureReply(t)
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, out, errOut := runApp("", "send", addr, example("0100-auth-request.hex"), "--format", "json")
	if code != 0 {
		t.Fatalf("send: code=%d err=%q", code, errOut)
	}
	// The literal PAN and its hex-encoded wire form must both be absent.
	if strings.Contains(out, "4111111111111111") {
		t.Errorf("default json leaked the literal PAN\n%s", out)
	}
	if strings.Contains(strings.ToUpper(out), panHex) {
		t.Errorf("default json leaked the hex-encoded PAN (%s) via a raw wire payload\n%s", panHex, out)
	}

	var payload struct {
		Request struct {
			Hex   string `json:"hex"`
			Bytes int    `json:"bytes"`
		} `json:"request"`
		Response struct {
			Hex   string `json:"hex"`
			Bytes int    `json:"bytes"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	// The byte counts stay (they are not sensitive); the raw hex is withheld.
	if payload.Request.Bytes == 0 || payload.Response.Bytes == 0 {
		t.Errorf("byte counts should be present by default: %+v", payload)
	}
	if payload.Request.Hex != "" || payload.Response.Hex != "" {
		t.Errorf("raw wire hex must be withheld by default: req=%q resp=%q", payload.Request.Hex, payload.Response.Hex)
	}
}

func TestSendJSONUnsafeRevealsWirePayload(t *testing.T) {
	t.Parallel()
	reply, panHex := panFixtureReply(t)
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, out, errOut := runApp("", "send", addr, example("0100-auth-request.hex"), "--format", "json", "--unsafe")
	if code != 0 {
		t.Fatalf("send: code=%d err=%q", code, errOut)
	}
	if !strings.Contains(strings.ToUpper(out), panHex) {
		t.Errorf("--unsafe json should include the raw wire payload hex (%s)\n%s", panHex, out)
	}
}

func TestSendRejectsInvalidFraming(t *testing.T) {
	t.Parallel()
	code, _, errOut := runApp("", "send", "127.0.0.1:1", example("0800-network-echo.hex"), "--framing", "bogus")
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "invalid --framing") {
		t.Errorf("error = %q, want invalid --framing", errOut)
	}
}

func TestSendRequiresAddress(t *testing.T) {
	t.Parallel()
	code, _, errOut := runApp("", "send")
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "Usage: iso8583tool send") {
		t.Errorf("missing usage on no address: %q", errOut)
	}
}

// startSilentServer accepts one connection, reads the framed request, then holds
// the connection open without ever replying, so the client's read deadline must
// fire. It exercises the timeout path over the real CLI.
func startSilentServer(t *testing.T, framing service.Framing) string {
	t.Helper()
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Drain the request but never reply; close only when the test ends.
		_, _ = framing.ReadResponse(conn)
		t.Cleanup(func() { _ = conn.Close() })
	}()
	return ln.Addr().String()
}

func TestSendNoneFramingSucceeds(t *testing.T) {
	t.Parallel()
	reply := sampleResponseBytes(t)
	addr := startEchoServer(t, service.FramingNone, reply)

	code, out, errOut := runApp("", "send", addr, example("0800-network-echo.hex"), "--framing", "none", "--format", "json")
	if code != 0 {
		t.Fatalf("send none: code=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, `"framing": "none"`) {
		t.Errorf("none framing not reported: %s", out)
	}
	if !strings.Contains(out, `"mti": "0810"`) {
		t.Errorf("none framing did not decode the 0810 response: %s", out)
	}
}

func TestSendNoneFramingTimesOut(t *testing.T) {
	t.Parallel()
	addr := startSilentServer(t, service.FramingNone)

	code, _, errOut := runApp("", "send", addr, example("0800-network-echo.hex"), "--framing", "none", "--timeout", "400ms")
	if code != 1 {
		t.Fatalf("code = %d, want 1 on timeout (err=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "timed out") {
		t.Errorf("expected a timeout error, got %q", errOut)
	}
}

func TestSendInlineJSONViaRaw(t *testing.T) {
	t.Parallel()
	reply := sampleResponseBytes(t)
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	// --raw takes an inline message; an inline JSON document is packed with the
	// active spec, proving --raw is not hex-only.
	code, out, errOut := runApp("", "send", addr, "--raw", `{"mti":"0800","fields":{"70":"301","11":"654321","41":"TERMNET1"}}`, "--format", "json")
	if code != 0 {
		t.Fatalf("send --raw json: code=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, `"mti": "0800"`) {
		t.Errorf("request_view should reflect the inline JSON 0800: %s", out)
	}
	if !strings.Contains(out, `"mti": "0810"`) {
		t.Errorf("response_view should decode the 0810 reply: %s", out)
	}
}

func TestSendExpectMTIAndFieldPass(t *testing.T) {
	t.Parallel()
	reply := sampleResponseBytes(t)
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, _, errOut := runApp("", "send", addr, example("0800-network-echo.hex"),
		"--expect-mti", "0810", "--expect-field", "39=00", "--expect-field", "70=301")
	if code != 0 {
		t.Fatalf("expectations should pass: code=%d err=%q", code, errOut)
	}
}

func TestSendExpectMTIMismatchFails(t *testing.T) {
	t.Parallel()
	reply := sampleResponseBytes(t)
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, _, errOut := runApp("", "send", addr, example("0800-network-echo.hex"), "--expect-mti", "0800")
	if code != 1 {
		t.Fatalf("code = %d, want 1 on MTI mismatch", code)
	}
	if !strings.Contains(errOut, "expectation failed") {
		t.Errorf("missing expectation-failed header: %q", errOut)
	}
	if !strings.Contains(errOut, "0800") || !strings.Contains(errOut, "0810") {
		t.Errorf("error should show expected (0800) vs actual (0810): %q", errOut)
	}
}

func TestSendExpectFieldMismatchFails(t *testing.T) {
	t.Parallel()
	reply := sampleResponseBytes(t)
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, _, errOut := runApp("", "send", addr, example("0800-network-echo.hex"), "--expect-field", "39=99")
	if code != 1 {
		t.Fatalf("code = %d, want 1 on field mismatch", code)
	}
	if !strings.Contains(errOut, "expectation failed") {
		t.Errorf("missing expectation-failed header: %q", errOut)
	}
}

func TestSendExpectFieldUnmaskedCanonical(t *testing.T) {
	t.Parallel()
	// The reply carries a real PAN; the assertion must compare against the
	// unmasked canonical value even though the printed view masks it.
	reply, _ := panFixtureReply(t)
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, out, errOut := runApp("", "send", addr, example("0100-auth-request.hex"),
		"--expect-field", "2=4111111111111111", "--format", "json")
	if code != 0 {
		t.Fatalf("expectation against the unmasked PAN should pass: code=%d err=%q", code, errOut)
	}
	// The default output still masks the PAN.
	if strings.Contains(out, "4111111111111111") {
		t.Errorf("output should still mask the PAN: %s", out)
	}
}

func TestSendExpectFieldRejectsMissingEquals(t *testing.T) {
	t.Parallel()
	code, _, errOut := runApp("", "send", "127.0.0.1:1", example("0800-network-echo.hex"), "--expect-field", "39")
	if code != 1 {
		t.Fatalf("code = %d, want 1 on malformed --expect-field", code)
	}
	if !strings.Contains(errOut, "invalid --expect-field") {
		t.Errorf("expected an invalid --expect-field error, got %q", errOut)
	}
}

func TestSendDryRunDescribeNeverConnects(t *testing.T) {
	t.Parallel()
	// Port 1 has nothing listening; a live send would fail to connect. --dry-run
	// must still succeed because it packs and frames without opening a connection.
	code, out, errOut := runApp("", "send", "127.0.0.1:1", example("0800-network-echo.hex"), "--dry-run", "--color", "never")
	if code != 0 {
		t.Fatalf("dry-run should not connect: code=%d err=%q", code, errOut)
	}
	for _, want := range []string{"Dry run", "Target:", "127.0.0.1:1", "Framing:", "Would send bytes:", "Request:", "0800"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run describe output missing %q\n%s", want, out)
		}
	}
}

func TestSendDryRunJSONShape(t *testing.T) {
	t.Parallel()
	code, out, errOut := runApp("", "send", "127.0.0.1:1", example("0800-network-echo.hex"), "--dry-run", "--format", "json")
	if code != 0 {
		t.Fatalf("dry-run json: code=%d err=%q", code, errOut)
	}
	var payload struct {
		DryRun         bool   `json:"dry_run"`
		Target         string `json:"target"`
		Framing        string `json:"framing"`
		WouldSendBytes int    `json:"would_send_bytes"`
		Request        struct {
			Hex   string `json:"hex"`
			Bytes int    `json:"bytes"`
		} `json:"request"`
		RequestView struct {
			MTI string `json:"mti"`
		} `json:"request_view"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal dry-run json: %v\n%s", err, out)
	}
	if !payload.DryRun {
		t.Error("dry_run should be true")
	}
	if payload.Target != "127.0.0.1:1" {
		t.Errorf("target = %q", payload.Target)
	}
	if payload.RequestView.MTI != "0800" {
		t.Errorf("request_view.mti = %q, want 0800", payload.RequestView.MTI)
	}
	// The framed total includes the 2-byte header, so it exceeds the payload bytes.
	if payload.WouldSendBytes <= payload.Request.Bytes {
		t.Errorf("would_send_bytes (%d) should exceed request.bytes (%d)", payload.WouldSendBytes, payload.Request.Bytes)
	}
	// Raw wire hex is withheld by default.
	if payload.Request.Hex != "" {
		t.Errorf("raw wire hex must be withheld without --unsafe: %q", payload.Request.Hex)
	}
}

func TestSendDryRunUnsafeRevealsWireHex(t *testing.T) {
	t.Parallel()
	_, panHex := panFixtureReply(t)
	code, out, errOut := runApp("", "send", "127.0.0.1:1", example("0100-auth-request.hex"), "--dry-run", "--format", "json", "--unsafe")
	if code != 0 {
		t.Fatalf("dry-run --unsafe json: code=%d err=%q", code, errOut)
	}
	if !strings.Contains(strings.ToUpper(out), panHex) {
		t.Errorf("--unsafe dry-run should include the raw request hex (%s)\n%s", panHex, out)
	}
}

func TestSendDryRunRejectsExpectations(t *testing.T) {
	t.Parallel()
	code, _, errOut := runApp("", "send", "127.0.0.1:1", example("0800-network-echo.hex"), "--dry-run", "--expect-mti", "0810")
	if code != 1 {
		t.Fatalf("code = %d, want 1 when --dry-run is combined with expectations", code)
	}
	if !strings.Contains(errOut, "dry-run") {
		t.Errorf("error should explain the dry-run conflict: %q", errOut)
	}
}

func TestSendRejectsInvalidAddress(t *testing.T) {
	t.Parallel()
	code, _, errOut := runApp("", "send", "127.0.0.1", example("0800-network-echo.hex"), "--timeout", "500ms")
	if code != 1 {
		t.Fatalf("code = %d, want 1 on a missing port", code)
	}
	if !strings.Contains(errOut, "invalid address") {
		t.Errorf("expected an invalid-address error, got %q", errOut)
	}
}

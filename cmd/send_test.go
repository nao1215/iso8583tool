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
		_, _ = conn.Write(framed)
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

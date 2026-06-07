package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/service"
)

// Issue 1: convert emits an unmasked document for round-trip fidelity, unlike
// view/diff/send. The help and runtime messaging must make that explicit so a
// user does not mistake convert output for the masked view output.

func TestConvertHelpWarnsUnmasked(t *testing.T) {
	t.Parallel()
	code, out, _ := runApp("", "convert", "--help")
	if code != 0 {
		t.Fatalf("convert --help: code=%d", code)
	}
	low := strings.ToLower(out)
	if !strings.Contains(low, "unmasked") {
		t.Errorf("convert help should state the output is unmasked\n%s", out)
	}
	if !strings.Contains(low, "round-trip") && !strings.Contains(low, "round trip") {
		t.Errorf("convert help should tie the unmasked output to round-trip fidelity\n%s", out)
	}
	if !strings.Contains(low, "redact") {
		t.Errorf("convert help should point to redact for safe sharing\n%s", out)
	}
}

func TestConvertOutputFileWarnsSensitive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.hex")
	code, stdout, errOut := runApp("", "convert", example("0100-auth-request.json"), "--output", outPath)
	if code != 0 {
		t.Fatalf("convert --output: code=%d err=%q", code, errOut)
	}
	// The human report and the sensitivity note both ride on stdout (the data went
	// to the file), so the note follows the report to /dev/null when discarded and
	// stderr stays clean for scripts.
	if !strings.Contains(stdout, "Converted with") {
		t.Errorf("convert --output should still report on stdout: %q", stdout)
	}
	low := strings.ToLower(stdout)
	if !strings.Contains(low, "unmasked") || !strings.Contains(low, "sensitive") {
		t.Errorf("convert --output should note the written file is unmasked/sensitive on stdout\n%s", stdout)
	}
	if errOut != "" {
		t.Errorf("convert --output must keep stderr clean: %q", errOut)
	}
}

func TestConvertPipedOutputStaysByteClean(t *testing.T) {
	t.Parallel()
	// stdout here is an in-memory buffer (not a TTY), so convert must stay silent
	// on stderr to keep convert | convert and scripted pipelines byte-clean.
	code, out, errOut := runApp("", "convert", example("0100-auth-request.hex"))
	if code != 0 {
		t.Fatalf("convert: code=%d err=%q", code, errOut)
	}
	if errOut != "" {
		t.Errorf("piped convert must not warn on stderr: %q", errOut)
	}
	if !strings.Contains(out, `"mti": "0100"`) {
		t.Errorf("convert stdout missing the document: %s", out)
	}
}

// Issue 2: send's describe output must surface the full set of present fields
// (not only the codes that decode to a human meaning) so a BASE I fault
// investigation does not miss F37/F38/F41/F42/F48/F63. It stays consistent with
// view: cardholder data is masked by default.

func TestSendDescribeShowsAllFields(t *testing.T) {
	t.Parallel()
	reply, _ := panFixtureReply(t) // 0110 auth response carrying F37/F38/F41/F42/F48/F63
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, out, errOut := runApp("", "send", addr, example("0100-auth-request.hex"), "--color", "never")
	if code != 0 {
		t.Fatalf("send: code=%d err=%q", code, errOut)
	}
	for _, want := range []string{
		"37 = REF123456789",
		"38 = A12345",
		"41 = TERMID01",
		"42 = MERCHANT0000001",
		"48 = APPROVED=Y|BALANCE=NA",
		"63 = NETWORKTRACE=OK",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("send describe missing field line %q\n%s", want, out)
		}
	}
	// The full listing must keep masking cardholder data, like view.
	if strings.Contains(out, "4111111111111111") {
		t.Errorf("send describe leaked an unmasked PAN\n%s", out)
	}
	if !strings.Contains(out, "411111******1111") {
		t.Errorf("send describe should show a masked PAN for F2\n%s", out)
	}
}

func TestSendUnsafeDescribeShowsRawFields(t *testing.T) {
	t.Parallel()
	reply, _ := panFixtureReply(t)
	addr := startEchoServer(t, service.Framing2ByteBinary, reply)

	code, out, errOut := runApp("", "send", addr, example("0100-auth-request.hex"), "--color", "never", "--unsafe")
	if code != 0 {
		t.Fatalf("send --unsafe: code=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, "2 = 4111111111111111") {
		t.Errorf("send --unsafe describe should show the raw PAN\n%s", out)
	}
}

// Issue 3: --dry-run is documented as a way to inspect the framed bytes before a
// real run. The framed wire bytes carry the payload in the clear, so they are
// withheld by default and revealed under --unsafe, matching the live-send hex.

func TestSendDryRunUnsafeShowsFramedBytesJSON(t *testing.T) {
	t.Parallel()
	code, out, errOut := runApp("", "send", "127.0.0.1:1", example("0800-network-echo.hex"),
		"--dry-run", "--format", "json", "--unsafe")
	if code != 0 {
		t.Fatalf("dry-run --unsafe json: code=%d err=%q", code, errOut)
	}
	var payload struct {
		WouldSendBytes int    `json:"would_send_bytes"`
		FramedHex      string `json:"framed_hex"`
		Request        struct {
			Hex string `json:"hex"`
		} `json:"request"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal dry-run json: %v\n%s", err, out)
	}
	if payload.FramedHex == "" {
		t.Fatalf("--unsafe dry-run json should include framed_hex\n%s", out)
	}
	// framed_hex is the whole wire frame (length header + payload), so its byte
	// length equals would_send_bytes and it ends with the request payload hex.
	if len(payload.FramedHex) != payload.WouldSendBytes*2 {
		t.Errorf("framed_hex (%d hex chars) should equal would_send_bytes*2 (%d)",
			len(payload.FramedHex), payload.WouldSendBytes*2)
	}
	if !strings.HasSuffix(strings.ToUpper(payload.FramedHex), strings.ToUpper(payload.Request.Hex)) {
		t.Errorf("framed_hex should end with the request payload hex\nframed=%s\npayload=%s",
			payload.FramedHex, payload.Request.Hex)
	}
}

func TestSendDryRunDefaultWithholdsFramedBytes(t *testing.T) {
	t.Parallel()
	code, out, errOut := runApp("", "send", "127.0.0.1:1", example("0800-network-echo.hex"),
		"--dry-run", "--format", "json")
	if code != 0 {
		t.Fatalf("dry-run json: code=%d err=%q", code, errOut)
	}
	if strings.Contains(out, "framed_hex") {
		t.Errorf("framed_hex must be withheld without --unsafe\n%s", out)
	}
}

func TestSendDryRunUnsafeDescribeShowsFramedBytes(t *testing.T) {
	t.Parallel()
	code, out, errOut := runApp("", "send", "127.0.0.1:1", example("0800-network-echo.hex"),
		"--dry-run", "--unsafe", "--color", "never")
	if code != 0 {
		t.Fatalf("dry-run --unsafe describe: code=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, "Framed bytes") {
		t.Errorf("dry-run --unsafe describe should print the framed bytes\n%s", out)
	}
}

func TestSendDryRunDefaultDescribeHidesFramedBytes(t *testing.T) {
	t.Parallel()
	code, out, errOut := runApp("", "send", "127.0.0.1:1", example("0800-network-echo.hex"),
		"--dry-run", "--color", "never")
	if code != 0 {
		t.Fatalf("dry-run describe: code=%d err=%q", code, errOut)
	}
	if strings.Contains(out, "Framed bytes") {
		t.Errorf("framed bytes must be withheld in describe without --unsafe\n%s", out)
	}
	if !strings.Contains(out, "Would send bytes:") {
		t.Errorf("dry-run describe should still report the byte count\n%s", out)
	}
}

func TestSendDryRunHelpDocumentsUnsafeFramedBytes(t *testing.T) {
	t.Parallel()
	code, out, _ := runApp("", "send", "--help")
	if code != 0 {
		t.Fatalf("send --help: code=%d", code)
	}
	low := strings.ToLower(out)
	if !strings.Contains(low, "framed") {
		t.Errorf("send help should describe the framed-bytes behaviour\n%s", out)
	}
	if !strings.Contains(low, "unsafe") {
		t.Errorf("send help should mention --unsafe reveals the framed bytes\n%s", out)
	}
}

// Issue 4: when basei-starter and spec87ascii fit equally well, doctor must
// explain how to choose between them (the Field 55 EMV overlay) instead of only
// asking the user to confirm by eye.

func TestDoctorAmbiguityExplainsFieldFiftyFive(t *testing.T) {
	t.Parallel()
	msg := filepath.Join("..", "examples", "spec87ascii", "0800-network-echo.hex")
	code, out, errOut := runApp("", "doctor", msg, "--no-color")
	if code != 0 {
		t.Fatalf("doctor: code=%d err=%q", code, errOut)
	}
	low := strings.ToLower(out)
	if !strings.Contains(low, "field 55") {
		t.Errorf("doctor should explain the basei-starter/spec87ascii split via Field 55\n%s", out)
	}
	// Both tied presets must still be named.
	if !strings.Contains(out, "basei-starter") || !strings.Contains(out, "spec87ascii") {
		t.Errorf("doctor should name both tied presets\n%s", out)
	}
}

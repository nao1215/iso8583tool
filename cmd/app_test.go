package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const examplesDir = "../examples/basei"

func example(name string) string { return filepath.Join(examplesDir, name) }

// runApp drives the CLI in-process with the given stdin and arguments.
func runApp(stdin string, args ...string) (code int, stdout, stderr string) {
	var out, errBuf bytes.Buffer
	app := NewApp(&out, &errBuf, strings.NewReader(stdin), ".")
	code = app.Run(args)
	return code, out.String(), errBuf.String()
}

func TestVersionAndHelp(t *testing.T) {
	t.Parallel()

	if code, out, _ := runApp("", "version"); code != 0 || !strings.Contains(out, "iso8583tool") {
		t.Fatalf("version: code=%d out=%q", code, out)
	}
	if code, _, errOut := runApp(""); code != 0 || !strings.Contains(errOut, "Commands:") {
		t.Fatalf("no-args help: code=%d err=%q", code, errOut)
	}
	if code, _, errOut := runApp("", "frobnicate"); code != 1 || !strings.Contains(errOut, "unknown command") {
		t.Fatalf("unknown command: code=%d err=%q", code, errOut)
	}
	if code, _, errOut := runApp("", "help", "convert"); code != 0 || !strings.Contains(errOut, "convert") {
		t.Fatalf("help convert: code=%d err=%q", code, errOut)
	}
}

func TestSample(t *testing.T) {
	t.Parallel()

	if code, out, _ := runApp("", "sample"); code != 0 || !strings.Contains(out, "0100-auth-request") {
		t.Fatalf("sample list: code=%d out=%q", code, out)
	}
	for _, want := range []string{
		"0200-financial-request",
		"0420-reversal-advice",
		"0800-network-echo",
	} {
		if code, out, _ := runApp("", "sample"); code != 0 || !strings.Contains(out, want) {
			t.Fatalf("sample list missing %q: code=%d out=%q", want, code, out)
		}
	}
	if code, out, _ := runApp("", "sample", "0100-auth-request", "--format", "hex"); code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("sample hex: code=%d out=%q", code, out)
	}
	if code, _, errOut := runApp("", "sample", "does-not-exist"); code != 1 || !strings.Contains(errOut, "unknown sample") {
		t.Fatalf("sample unknown: code=%d err=%q", code, errOut)
	}

	out := filepath.Join(t.TempDir(), "s.json")
	if code, _, _ := runApp("", "sample", "0110-auth-response", "--output", out); code != 0 {
		t.Fatalf("sample --output failed: %d", code)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("sample --output did not write file: %v", err)
	}
}

func TestViewDescribeDecodesAndMasks(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "view", example("0110-auth-response.hex"))
	if code != 0 {
		t.Fatalf("view failed: %d", code)
	}
	for _, want := range []string{
		"Summary:", "Approved", "JPY 5000", // human-readable summary
		"4111****1111",                   // PAN masked
		"06-04 12:34:56",                 // date decoded
		"Authorization Request response", // MTI decoded
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view output missing %q\n%s", want, out)
		}
	}
}

func TestViewJSONHasDecodedAndNoColor(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "view", example("0110-auth-response.hex"), "--format", "json", "--color", "always")
	if code != 0 {
		t.Fatalf("view json failed: %d", code)
	}
	if !strings.Contains(out, "\"decoded\"") {
		t.Fatalf("json missing decoded array:\n%s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("json must never be colorized:\n%s", out)
	}
}

func TestViewFilter(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "view", example("0110-auth-response.hex"), "--filter", "39", "--filter", "55.8A")
	if code != 0 {
		t.Fatalf("view --filter failed: %d", code)
	}
	if !strings.Contains(out, "Approved") || !strings.Contains(out, "55.8A") {
		t.Fatalf("filter output unexpected:\n%s", out)
	}
	if strings.Contains(out, "Primary Account Number") {
		t.Fatalf("filter should not print unrelated fields:\n%s", out)
	}
}

func TestViewFromStdin(t *testing.T) {
	t.Parallel()

	_, hex, _ := runApp("", "sample", "0110-auth-response", "--format", "hex")
	code, out, _ := runApp(hex, "view", "-")
	if code != 0 || !strings.Contains(out, "MTI") {
		t.Fatalf("view from stdin: code=%d out=%q", code, out)
	}
}

func TestViewNetworkSampleShowsNMICMeaning(t *testing.T) {
	t.Parallel()

	_, hex, _ := runApp("", "sample", "0800-network-echo", "--format", "hex")
	code, out, _ := runApp(hex, "view", "-")
	if code != 0 {
		t.Fatalf("view network sample failed: %d", code)
	}
	for _, want := range []string{"Echo test", "0800", "TERMNET1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("network view output missing %q\n%s", want, out)
		}
	}
}

func TestFlagsAfterPositional(t *testing.T) {
	t.Parallel()

	// The positional target may appear before the flags.
	code, out, _ := runApp("", "view", example("0110-auth-response.hex"), "--format", "json")
	if code != 0 || !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("flags after positional: code=%d out=%q", code, out)
	}
}

func TestColorModes(t *testing.T) {
	t.Parallel()

	_, always, _ := runApp("", "view", example("0110-auth-response.hex"), "--color", "always")
	if !strings.Contains(always, "\x1b[") {
		t.Fatal("--color always should emit ANSI escapes")
	}
	_, none, _ := runApp("", "view", example("0110-auth-response.hex"), "--no-color")
	if strings.Contains(none, "\x1b[") {
		t.Fatal("--no-color should not emit ANSI escapes")
	}
}

func TestConvertRoundTripCLI(t *testing.T) {
	t.Parallel()

	_, hex, _ := runApp("", "sample", "0100-auth-request", "--format", "hex")
	code, jsonDoc, _ := runApp(hex, "convert", "-") // hex -> json
	if code != 0 || !strings.Contains(jsonDoc, "\"mti\"") {
		t.Fatalf("convert hex->json: code=%d out=%q", code, jsonDoc)
	}
	code, back, _ := runApp(jsonDoc, "convert", "-") // json -> hex
	if code != 0 || strings.TrimSpace(back) == "" {
		t.Fatalf("convert json->hex: code=%d out=%q", code, back)
	}
	if strings.TrimSpace(back) != strings.TrimSpace(hex) {
		t.Fatalf("round-trip not stable:\n%q\nvs\n%q", back, hex)
	}
}

func TestConvertForceDirection(t *testing.T) {
	t.Parallel()

	if code, _, _ := runApp("", "convert", example("0100-auth-request.json"), "--to", "hex"); code != 0 {
		t.Fatalf("convert --to hex failed: %d", code)
	}
	if code, _, errOut := runApp("", "convert", example("0100-auth-request.json"), "--to", "bogus"); code != 1 || !strings.Contains(errOut, "unsupported --to") {
		t.Fatalf("convert bad --to: code=%d err=%q", code, errOut)
	}
}

func TestValidateOKAndBroken(t *testing.T) {
	t.Parallel()

	if code, out, _ := runApp("", "validate", example("0110-auth-response.hex")); code != 0 || !strings.Contains(out, "Validation: ok") {
		t.Fatalf("validate ok: code=%d out=%q", code, out)
	}
	// A broken message exits 1 and names the field that failed.
	code, out, _ := runApp("", "validate", "--raw", "01007220")
	if code != 1 {
		t.Fatalf("broken validate should exit 1, got %d", code)
	}
	if !strings.Contains(out, "[error]") || !strings.Contains(out, "input was") {
		t.Fatalf("broken validate should explain where: %q", out)
	}
}

func TestValidateJSON(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "validate", example("0110-auth-response.hex"), "--format", "json")
	if code != 0 || !strings.Contains(out, "\"valid\"") || !strings.Contains(out, "\"summary\"") {
		t.Fatalf("validate json: code=%d out=%q", code, out)
	}
}

func TestConvertToFile(t *testing.T) {
	t.Parallel()

	out := filepath.Join(t.TempDir(), "out.hex")
	code, stdout, _ := runApp("", "convert", example("0100-auth-request.json"), "--output", out)
	if code != 0 || !strings.Contains(stdout, "Converted with") {
		t.Fatalf("convert --output: code=%d out=%q", code, stdout)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("convert --output did not write file: %v", err)
	}
}

func TestConfigOverride(t *testing.T) {
	t.Parallel()

	cfg := filepath.Join(t.TempDir(), "cfg.json")
	body := `{"spec":"basei-starter","extensions":[{"id":63,"name":"Acme Blob","strategy":"opaque"}]}`
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runApp("", "validate", example("0110-auth-response.hex"), "--config", cfg)
	if code != 0 || !strings.Contains(out, "Acme Blob") {
		t.Fatalf("config override: code=%d out=%q", code, out)
	}

	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte(`{"extensions":[{"id":1,"strategy":"nope"}]}`), 0o600)
	if code, _, errOut := runApp("", "view", example("0110-auth-response.hex"), "--config", bad); code != 1 || !strings.Contains(errOut, "strategy") {
		t.Fatalf("bad config should fail: code=%d err=%q", code, errOut)
	}
}

func TestConvertLongUnknownTag(t *testing.T) {
	t.Parallel()

	// A value over 127 bytes forces long-form BER-TLV length encoding.
	value := strings.Repeat("AB", 130)
	doc := `{"mti":"0100","fields":{"11":"123456"},"binary_fields":{"55.DF01":"` + value + `"}}`

	code, hex, _ := runApp(doc, "convert", "-")
	if code != 0 || strings.TrimSpace(hex) == "" {
		t.Fatalf("convert long tag: code=%d out=%q", code, hex)
	}
	code, back, _ := runApp(hex, "convert", "-")
	if code != 0 || !strings.Contains(strings.ToUpper(back), strings.ToUpper(value)) {
		t.Fatalf("long unknown tag not preserved: code=%d\n%s", code, back)
	}
}

func TestInputSelectionErrors(t *testing.T) {
	t.Parallel()

	// Both a file and --raw is ambiguous.
	if code, _, errOut := runApp("", "view", example("0110-auth-response.hex"), "--raw", "0100"); code != 1 || errOut == "" {
		t.Fatalf("file+raw should error: code=%d err=%q", code, errOut)
	}
	// Bad hex is reported.
	if code, _, errOut := runApp("", "view", "--raw", "zzzz"); code != 1 || !strings.Contains(errOut, "hex") {
		t.Fatalf("bad hex: code=%d err=%q", code, errOut)
	}
}

func TestReorderEndOfOptions(t *testing.T) {
	t.Parallel()

	// Everything after "--" stays positional, re-emitted after a "--" marker so
	// the flag parser does not treat "-response.hex" as a flag.
	got := reorder([]string{"--", "-response.hex"}, viewValueFlags)
	if len(got) != 2 || got[0] != "--" || got[1] != "-response.hex" {
		t.Fatalf("reorder(-- -response.hex) = %#v", got)
	}

	// A positional before flags is moved behind a "--" so it is never reparsed.
	got = reorder([]string{"msg", "--filter", "39"}, viewValueFlags)
	want := []string{"--filter", "39", "--", "msg"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("reorder = %#v, want %#v", got, want)
	}

	// No positionals: no trailing "--" is added.
	got = reorder([]string{"--format", "json"}, viewValueFlags)
	if strings.Join(got, " ") != "--format json" {
		t.Fatalf("reorder(no positional) = %#v", got)
	}
}

//nolint:paralleltest // uses t.Chdir, which forbids t.Parallel()
func TestEndOfOptionsDashFilename(t *testing.T) {
	_, hexData, _ := runApp("", "sample", "0110-auth-response", "--format", "hex")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "-response.hex"), []byte(hexData), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	code, out, errOut := runApp("", "view", "--", "-response.hex")
	if code != 0 {
		t.Fatalf("view -- -response.hex failed: code=%d err=%q", code, errOut)
	}
	if strings.Contains(errOut, "flag provided but not defined") {
		t.Fatalf("dash-leading filename was parsed as a flag: %q", errOut)
	}
	if !strings.Contains(out, "MTI") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRootHelpVersionRejectExtraArgs(t *testing.T) {
	t.Parallel()

	// Root --help / --version must not silently ignore trailing arguments.
	if code, _, errOut := runApp("", "--help", "view"); code != 1 || !strings.Contains(errOut, "takes no arguments") {
		t.Fatalf("--help view: code=%d err=%q", code, errOut)
	}
	if code, _, errOut := runApp("", "--version", "view"); code != 1 || !strings.Contains(errOut, "takes no arguments") {
		t.Fatalf("--version view: code=%d err=%q", code, errOut)
	}
	if code, _, _ := runApp("", "-h", "extra"); code != 1 {
		t.Fatal("-h extra should fail")
	}

	// Plain forms and the help subcommand keep working.
	if code, _, _ := runApp("", "--help"); code != 0 {
		t.Fatal("--help should succeed")
	}
	if code, out, _ := runApp("", "--version"); code != 0 || !strings.Contains(out, "iso8583tool") {
		t.Fatal("--version should print the version")
	}
	if code, _, errOut := runApp("", "help", "view"); code != 0 || !strings.Contains(errOut, "Usage: iso8583tool view") {
		t.Fatalf("help view should still describe view: code=%d err=%q", code, errOut)
	}
}

func TestInvalidColorValue(t *testing.T) {
	t.Parallel()

	for _, cmd := range []string{"view", "validate"} {
		code, _, errOut := runApp("", cmd, example("0110-auth-response.hex"), "--color", "banana")
		if code != 1 || !strings.Contains(errOut, "invalid --color") {
			t.Fatalf("%s --color banana: code=%d err=%q", cmd, code, errOut)
		}
	}
}

func TestConvertInvalidDocuments(t *testing.T) {
	t.Parallel()

	// A document with an invalid field id fails to pack.
	bad := `{"mti":"0100","binary_fields":{"55.ZZ":"00"}}`
	if code, _, errOut := runApp(bad, "convert", "-", "--to", "hex"); code != 1 || errOut == "" {
		t.Fatalf("invalid TLV tag should fail: code=%d err=%q", code, errOut)
	}
	// A document missing mti is rejected.
	if code, _, errOut := runApp(`{"fields":{}}`, "convert", "-", "--to", "hex"); code != 1 || !strings.Contains(errOut, "mti") {
		t.Fatalf("missing mti should fail: code=%d err=%q", code, errOut)
	}
}

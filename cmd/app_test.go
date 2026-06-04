package cmd

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
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
		"411111******1111",                     // PAN masked (BIN + last four, same as JSON/redact)
		"06-04 12:34:56",                       // date decoded
		"Authorization Response from Acquirer", // MTI decoded (natural response wording)
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view output missing %q\n%s", want, out)
		}
	}
}

func TestViewShowsEMVDotPaths(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "view", example("0100-auth-request.hex"), "--no-color")
	if code != 0 {
		t.Fatalf("view failed: %d", code)
	}
	// Field 55 subtags render with their full dot-path and EMV tag name, so the
	// path is copy-pasteable for --filter and editing.
	for _, want := range []string{
		"55.9F26", "Application Cryptogram",
		"55.82", "Application Interchange Profile",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view output missing %q\n%s", want, out)
		}
	}
}

func TestViewJSONMasksCardholderData(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "view", example("0100-auth-request.hex"), "--format", "json")
	if code != 0 {
		t.Fatalf("view json failed: %d", code)
	}
	// The full PAN appears in field 2 and track 2; neither may leak.
	if strings.Contains(out, "4111111111111111") {
		t.Fatalf("view --format json leaked the PAN:\n%s", out)
	}
	if !strings.Contains(out, "411111******1111") {
		t.Fatalf("expected a masked PAN in view json:\n%s", out)
	}
}

func TestViewFilterExpandsComposite(t *testing.T) {
	t.Parallel()

	// A filter on a composite root must expand into child paths, not dump raw
	// bytes (matching diff --filter).
	code, out, _ := runApp("", "view", example("0110-auth-response.hex"), "--filter", "55", "--format", "json")
	if code != 0 {
		t.Fatalf("view --filter failed: %d", code)
	}
	if !strings.Contains(out, "55.8A") {
		t.Fatalf("filter on composite 55 should expand to subpaths:\n%s", out)
	}
	if strings.Contains(out, "\"path\": \"55\"") {
		t.Fatalf("filter should not emit the raw composite root:\n%s", out)
	}
}

func TestViewFilterJSONObjectContract(t *testing.T) {
	t.Parallel()

	// Filtered JSON must be object-shaped (like the unfiltered view), with only
	// the matched fields, and an explicit missing_filters list so a typo or an
	// absent field is distinguishable.
	code, out, _ := runApp("", "view", example("0110-auth-response.hex"),
		"--filter", "39", "--filter", "55.8A", "--filter", "90", "--format", "json")
	if code != 0 {
		t.Fatalf("filtered json failed: %d", code)
	}
	if trimmed := strings.TrimSpace(out); !strings.HasPrefix(trimmed, "{") {
		t.Fatalf("filtered json must be an object, got:\n%s", out)
	}

	var payload struct {
		MTI          string            `json:"mti"`
		Fields       map[string]string `json:"fields"`
		BinaryFields map[string]string `json:"binary_fields"`
		Summary      string            `json:"summary"`
		Decoded      []struct {
			Path    string `json:"path"`
			Meaning string `json:"meaning"`
		} `json:"decoded"`
		MissingFilters []string `json:"missing_filters"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("filtered json is not valid object json: %v\n%s", err, out)
	}
	if payload.MTI != "0110" {
		t.Fatalf("mti = %q, want 0110", payload.MTI)
	}
	if _, ok := payload.Fields["39"]; !ok {
		t.Fatalf("fields should contain matched F39, got %#v", payload.Fields)
	}
	if _, ok := payload.BinaryFields["55.8A"]; !ok {
		t.Fatalf("binary_fields should contain matched 55.8A, got %#v", payload.BinaryFields)
	}
	if _, ok := payload.Fields["2"]; ok {
		t.Fatalf("filtered json must not include unmatched fields like F2: %#v", payload.Fields)
	}
	if len(payload.MissingFilters) != 1 || payload.MissingFilters[0] != "90" {
		t.Fatalf("missing_filters should be [90], got %#v", payload.MissingFilters)
	}
	// Consistent with the unfiltered view --format json: summary and decoded
	// meanings are carried over (filtered to the matched paths).
	if payload.Summary == "" {
		t.Fatalf("filtered json should carry a summary like the unfiltered view:\n%s", out)
	}
	foundMeaning := false
	for _, d := range payload.Decoded {
		if d.Path == "39" && d.Meaning == "Approved" {
			foundMeaning = true
		}
	}
	if !foundMeaning {
		t.Fatalf("filtered json decoded should include F39 -> Approved:\n%s", out)
	}
}

// TestViewFilterJSONMissingFiltersAlwaysArray pins that missing_filters is always
// present as an array (never null/absent), so jq pipelines are stable.
func TestViewFilterJSONMissingFiltersAlwaysArray(t *testing.T) {
	t.Parallel()

	// All filters match -> missing_filters must still be present as [].
	code, out, _ := runApp("", "view", example("0110-auth-response.hex"), "--filter", "39", "--format", "json")
	if code != 0 {
		t.Fatalf("filtered json failed: %d", code)
	}
	if !strings.Contains(out, "\"missing_filters\": []") {
		t.Fatalf("missing_filters must be present as an empty array when nothing is missing:\n%s", out)
	}
}

func TestRedactTextFieldOrder(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "redact", example("0100-auth-request.hex"), "--format", "text", "--color", "never")
	if code != 0 {
		t.Fatalf("redact text failed: %d", code)
	}
	// Fields must be in numeric order (F2, F3, F4, F7, F11, ...), not dictionary
	// order (which would put F11 before F2).
	order := []string{"F2 =", "F3 =", "F4 =", "F7 =", "F11 =", "F22 =", "F49 ="}
	prev := -1
	for _, marker := range order {
		idx := strings.Index(out, marker)
		if idx < 0 {
			t.Fatalf("missing %q in:\n%s", marker, out)
		}
		if idx < prev {
			t.Fatalf("%q is out of numeric order:\n%s", marker, out)
		}
		prev = idx
	}
}

func TestHelpUsageStrings(t *testing.T) {
	t.Parallel()

	_, _, redactHelp := runApp("", "help", "redact")
	if !strings.Contains(redactHelp, "[--raw HEX]") {
		t.Fatalf("redact usage should document --raw:\n%s", redactHelp)
	}
	if !strings.Contains(redactHelp, "[--color auto|always|never]") {
		t.Fatalf("redact usage should document --color:\n%s", redactHelp)
	}

	_, _, validateHelp := runApp("", "help", "validate")
	if !strings.Contains(validateHelp, "[--raw HEX]") {
		t.Fatalf("validate usage should document --raw:\n%s", validateHelp)
	}

	_, _, convertHelp := runApp("", "help", "convert")
	if strings.Contains(convertHelp, "packed BASE I message") {
		t.Fatalf("convert help should not imply BASE I is the only spec:\n%s", convertHelp)
	}
	if !strings.Contains(convertHelp, "--spec") {
		t.Fatalf("convert help should mention --spec for selecting the spec:\n%s", convertHelp)
	}
	if !strings.Contains(convertHelp, "--config") {
		t.Fatalf("convert help should still mention --config:\n%s", convertHelp)
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

func TestViewRawPackedBCDMessage(t *testing.T) {
	t.Parallel()

	raw := kanmuLikeRaw(t)
	code, out, errOut := runApp(string(raw), "view", "-", "--encoding", "raw", "--spec", "spec87bcd-starter", "--color", "never")
	if code != 0 {
		t.Fatalf("view raw packed bcd: code=%d err=%q", code, errOut)
	}
	for _, want := range []string{
		"401924******9999",
		"327327",
		"1138",
		"2204",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("view raw packed bcd missing %q:\n%s", want, out)
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

func TestConvertRoundTripRawPackedBCDCLI(t *testing.T) {
	t.Parallel()

	raw := kanmuLikeRaw(t)
	code, jsonDoc, errOut := runApp(string(raw), "convert", "-", "--encoding", "raw", "--spec", "spec87bcd-starter")
	if code != 0 {
		t.Fatalf("convert raw->json: code=%d err=%q", code, errOut)
	}
	if !strings.Contains(jsonDoc, `"mti": "0100"`) || !strings.Contains(jsonDoc, `"4": "000000001138"`) {
		t.Fatalf("convert raw->json missing fields:\n%s", jsonDoc)
	}

	code, back, errOut := runApp(jsonDoc, "convert", "-", "--to", "hex", "--encoding", "raw", "--spec", "spec87bcd-starter")
	if code != 0 {
		t.Fatalf("convert json->raw: code=%d err=%q", code, errOut)
	}
	if back != string(raw) {
		t.Fatalf("raw round-trip not stable:\n%x\nvs\n%x", []byte(back), raw)
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

func TestSpecFlagOverridesConfigSpec(t *testing.T) {
	t.Parallel()

	cfg := filepath.Join(t.TempDir(), "cfg.json")
	body := `{"spec":"basei-starter"}`
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	raw := kanmuLikeRaw(t)
	code, out, errOut := runApp(string(raw), "view", "-", "--encoding", "raw", "--config", cfg, "--spec", "spec87bcd-starter", "--color", "never")
	if code != 0 {
		t.Fatalf("spec override should win: code=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, "401924******9999") {
		t.Fatalf("spec override output unexpected:\n%s", out)
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

	valueFlags := map[string]bool{"filter": true, "format": true}

	// Everything after "--" stays positional, re-emitted after a "--" marker so
	// the flag parser does not treat "-response.hex" as a flag.
	got := reorder([]string{"--", "-response.hex"}, valueFlags)
	if len(got) != 2 || got[0] != "--" || got[1] != "-response.hex" {
		t.Fatalf("reorder(-- -response.hex) = %#v", got)
	}

	// A positional before flags is moved behind a "--" so it is never reparsed.
	got = reorder([]string{"msg", "--filter", "39"}, valueFlags)
	want := []string{"--filter", "39", "--", "msg"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("reorder = %#v, want %#v", got, want)
	}

	// No positionals: no trailing "--" is added.
	got = reorder([]string{"--format", "json"}, valueFlags)
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

func TestDiffCommand(t *testing.T) {
	t.Parallel()

	req := example("0100-auth-request.hex")
	resp := example("0110-auth-response.hex")

	// Different messages produce changes; exit 0.
	code, out, _ := runApp("", "diff", req, resp)
	if code != 0 || !strings.Contains(out, "changed") {
		t.Fatalf("diff: code=%d out=%q", code, out)
	}
	// Identical messages report no differences.
	if code, out, _ := runApp("", "diff", resp, resp); code != 0 || !strings.Contains(out, "No differences.") {
		t.Fatalf("diff identical: code=%d out=%q", code, out)
	}
	// JSON output is structured.
	if code, out, _ := runApp("", "diff", req, resp, "--format", "json"); code != 0 || !strings.Contains(out, "\"changes\"") {
		t.Fatalf("diff json: code=%d out=%q", code, out)
	}
	// One side may be stdin.
	_, hexData, _ := runApp("", "sample", "0110-auth-response", "--format", "hex")
	if code, out, _ := runApp(hexData, "diff", req, "-"); code != 0 || out == "" {
		t.Fatalf("diff stdin: code=%d out=%q", code, out)
	}
	// Two stdin sides is an error.
	if code, _, errOut := runApp("", "diff", "-", "-"); code != 1 || !strings.Contains(errOut, "stdin") {
		t.Fatalf("diff two stdin: code=%d err=%q", code, errOut)
	}
	// Wrong arity.
	if code, _, _ := runApp("", "diff", req); code != 1 {
		t.Fatal("diff with one arg should fail")
	}
}

func TestDiffMasksSensitiveDataByDefault(t *testing.T) {
	t.Parallel()

	req := example("0100-auth-request.hex")   // carries track 2 in field 35
	resp := example("0110-auth-response.hex") // has no field 35

	// By default diff is as safe to share as view/redact: the removed track 2
	// must not appear in full.
	code, out, _ := runApp("", "diff", req, resp, "--color", "never")
	if code != 0 {
		t.Fatalf("diff: code=%d out=%q", code, out)
	}
	if strings.Contains(out, "4111111111111111D") {
		t.Fatalf("diff leaked full track 2 by default:\n%s", out)
	}

	// --unsafe is an explicit opt-in to the raw values.
	code, outU, _ := runApp("", "diff", req, resp, "--color", "never", "--unsafe")
	if code != 0 {
		t.Fatalf("diff --unsafe: code=%d", code)
	}
	if !strings.Contains(outU, "4111111111111111D") {
		t.Fatalf("diff --unsafe should reveal the raw track 2:\n%s", outU)
	}
}

func TestDiffRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	req := example("0100-auth-request.hex")
	resp := example("0110-auth-response.hex")
	code, _, errOut := runApp("", "diff", req, resp, "--format", "bogus")
	if code != 1 || !strings.Contains(errOut, "unsupported format") {
		t.Fatalf("diff bogus format: code=%d err=%q", code, errOut)
	}
}

func TestValidateStrictFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "hollow.json")
	if err := os.WriteFile(jsonPath, []byte(`{"mti":"0110","fields":{"11":"123456"}}`), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	hexPath := filepath.Join(dir, "hollow.hex")
	if code, _, errOut := runApp("", "convert", jsonPath, "--to", "hex", "--output", hexPath); code != 0 {
		t.Fatalf("convert: code=%d err=%q", code, errOut)
	}

	// Lenient validation only checks that the message unpacks.
	if code, out, _ := runApp("", "validate", hexPath, "--color", "never"); code != 0 || !strings.Contains(out, "ok") {
		t.Fatalf("lenient validate: code=%d out=%q", code, out)
	}

	// --strict flags the hollow response and exits non-zero.
	code, out, _ := runApp("", "validate", hexPath, "--strict", "--color", "never")
	if code != 1 || !strings.Contains(out, "failed") || !strings.Contains(out, "39") {
		t.Fatalf("strict validate: code=%d out=%q", code, out)
	}
}

func TestOverlayConfigRelabelsPrivateFields(t *testing.T) {
	t.Parallel()

	overlay := filepath.Join(examplesDir, "..", "basei-overlay-config.json")
	// 0110-auth-response carries private fields 48 and 63, so the overlay's
	// custom names and strategies must surface in the describe view.
	code, out, _ := runApp("", "view", example("0110-auth-response.hex"),
		"--config", overlay, "--color", "never")
	if code != 0 {
		t.Fatalf("view with overlay: code=%d out=%q", code, out)
	}
	for _, want := range []string{"Acme Settlement Trace", "Acme Loyalty Segment"} {
		if !strings.Contains(out, want) {
			t.Fatalf("overlay name %q missing from describe:\n%s", want, out)
		}
	}
}

func TestPrivateFieldPANIsSafeByDefault(t *testing.T) {
	t.Parallel()

	doc := `{"mti":"0110","fields":{"11":"123456","39":"00","63":"PAN=4111111111111111"}}`
	hexOut := func(t *testing.T) string {
		t.Helper()
		code, out, errOut := runApp(doc, "convert", "-", "--to", "hex")
		if code != 0 {
			t.Fatalf("convert: code=%d err=%q", code, errOut)
		}
		return strings.TrimSpace(out)
	}

	// view, redact: default must not leak; view --unsafe reveals.
	hx := hexOut(t)
	if code, out, _ := runApp(hx, "view", "-", "--format", "json", "--color", "never"); code != 0 || strings.Contains(out, "4111111111111111") {
		t.Fatalf("view default leaked PAN: code=%d out=%q", code, out)
	}
	if code, out, _ := runApp(hx, "view", "-", "--format", "json", "--color", "never", "--unsafe"); code != 0 || !strings.Contains(out, "4111111111111111") {
		t.Fatalf("view --unsafe should reveal PAN: code=%d out=%q", code, out)
	}
	if code, out, _ := runApp(hx, "redact", "-"); code != 0 || strings.Contains(out, "4111111111111111") {
		t.Fatalf("redact default leaked PAN: code=%d out=%q", code, out)
	}
}

func TestConvertRejectsConflictingDocument(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  string
		want string
	}{
		{"dup path", `{"mti":"0100","fields":{"55.8A":"00"},"binary_fields":{"55.8A":"3035"}}`, "55.8A"},
		{"parent child tlv", `{"mti":"0100","binary_fields":{"55":"9F0206000000005000","55.9F02":"000000009999"}}`, "55.9F02"},
		{"parent child positional", `{"mti":"0100","fields":{"48":"x","48.1":"y"}}`, "48.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, _, errOut := runApp(tc.doc, "convert", "-", "--to", "hex")
			if code != 1 {
				t.Fatalf("conflicting document should fail: code=%d", code)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Fatalf("error should name %q, got %q", tc.want, errOut)
			}
		})
	}
}

func TestRedactCommand(t *testing.T) {
	t.Parallel()

	req := example("0100-auth-request.hex")

	// JSON (default) masks the PAN and never leaks it.
	code, out, _ := runApp("", "redact", req)
	if code != 0 || !strings.Contains(out, "411111******1111") || strings.Contains(out, "4111111111111111") {
		t.Fatalf("redact json: code=%d out=%q", code, out)
	}
	// Text format lists redacted paths.
	if code, out, _ := runApp("", "redact", req, "--format", "text"); code != 0 || !strings.Contains(out, "Redacted:") {
		t.Fatalf("redact text: code=%d out=%q", code, out)
	}
	// stdin works.
	_, hexData, _ := runApp("", "sample", "0100-auth-request", "--format", "hex")
	if code, out, _ := runApp(hexData, "redact", "-"); code != 0 || strings.Contains(out, "4111111111111111") {
		t.Fatalf("redact stdin: code=%d out=%q", code, out)
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

func kanmuLikeRaw(t *testing.T) []byte {
	t.Helper()

	raw, err := hex.DecodeString("010070040000000000001040192499999999993273270000000011382204")
	if err != nil {
		t.Fatalf("decode kanmu-like raw: %v", err)
	}
	return raw
}

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpecsLists(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "specs")
	if code != 0 {
		t.Fatalf("specs failed: %d", code)
	}
	for _, want := range []string{
		"basei-starter (default)",
		"spec87ascii",
		"spec87bcd-starter",
		"doctor MESSAGE",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("specs output missing %q\n%s", want, out)
		}
	}
}

func TestSpecsJSON(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "specs", "--format", "json")
	if code != 0 {
		t.Fatalf("specs json failed: %d", code)
	}
	var presets []struct {
		Name    string `json:"name"`
		Default bool   `json:"default"`
	}
	if err := json.Unmarshal([]byte(out), &presets); err != nil {
		t.Fatalf("specs json not parseable: %v\n%s", err, out)
	}
	if len(presets) != 3 {
		t.Fatalf("want 3 presets, got %d", len(presets))
	}
	if presets[0].Name != "basei-starter" || !presets[0].Default {
		t.Errorf("first preset should be the default basei-starter, got %+v", presets[0])
	}
}

func TestDoctorRecommendsStarter(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "doctor", example("0110-auth-response.hex"), "--no-color")
	if code != 0 {
		t.Fatalf("doctor failed: %d\n%s", code, out)
	}
	for _, want := range []string{
		"Recommended: --spec basei-starter",
		"recommended",
		"Confirm with: iso8583tool view",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q\n%s", want, out)
		}
	}
}

func TestDoctorDetectsPackedBCDFromRaw(t *testing.T) {
	t.Parallel()

	// kanmu-style packed-BCD message passed inline as hex.
	code, out, _ := runApp("", "doctor", "--raw",
		"010070040000000000001040192499999999993273270000000011382204", "--no-color")
	if code != 0 {
		t.Fatalf("doctor bcd failed: %d\n%s", code, out)
	}
	if !strings.Contains(out, "Recommended: --spec spec87bcd-starter") {
		t.Errorf("doctor should recommend the BCD preset\n%s", out)
	}
}

func TestDoctorAutoDetectsRawBinFile(t *testing.T) {
	t.Parallel()

	// A raw-binary *.bin capture must work without the user knowing to pass
	// --encoding raw: doctor auto-detects the input encoding.
	raw := []byte{
		0x01, 0x00, 0x70, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x10, 0x40, 0x19, 0x24, 0x99, 0x99, 0x99, 0x99, 0x99, 0x32,
		0x73, 0x27, 0x00, 0x00, 0x00, 0x00, 0x11, 0x38, 0x22, 0x04,
	}
	path := filepath.Join(t.TempDir(), "message.bin")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	code, out, errOut := runApp("", "doctor", path, "--no-color")
	if code != 0 {
		t.Fatalf("doctor on .bin failed: %d\nstdout=%s\nstderr=%s", code, out, errOut)
	}
	for _, want := range []string{
		"(raw input)",
		"Recommended: --spec spec87bcd-starter",
		"--encoding raw", // confirm hint must round-trip
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor .bin output missing %q\n%s", want, out)
		}
	}
}

func TestDoctorAutoDetectsRawAsciiAllNumeric(t *testing.T) {
	t.Parallel()

	// A raw ASCII 0800 message whose every byte is a digit is, byte-for-byte, a
	// valid even-length hex string, so the cheap "looks like hex" test cannot
	// tell the two readings apart. Auto-detection must still pick the raw
	// reading, which round-trips exactly through basei-starter, instead of
	// decoding it as packed hex and recommending the wrong BCD preset.
	raw := "0800022000000000000000000000000000000604161616654321"
	path := filepath.Join(t.TempDir(), "ascii.bin")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	code, out, errOut := runApp("", "doctor", path, "--no-color")
	if code != 0 {
		t.Fatalf("doctor on raw ASCII failed: %d\nstdout=%s\nstderr=%s", code, out, errOut)
	}
	for _, want := range []string{
		"(raw input)",
		"Recommended: --spec basei-starter",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor raw ASCII output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "Recommended: --spec spec87bcd-starter") {
		t.Errorf("doctor must not mis-read a raw ASCII message as packed hex\n%s", out)
	}
}

func TestDoctorJSONAndNoFit(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "doctor", "--raw", "fffefd", "--format", "json")
	if code != 1 {
		t.Fatalf("doctor on garbage should exit 1, got %d\n%s", code, out)
	}
	var diag struct {
		Recommended string `json:"recommended"`
		Candidates  []struct {
			Unpacks bool `json:"unpacks"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &diag); err != nil {
		t.Fatalf("doctor json not parseable: %v\n%s", err, out)
	}
	if diag.Recommended != "" {
		t.Errorf("garbage should have no recommendation, got %q", diag.Recommended)
	}
}

func TestValidateFailureHintsDoctor(t *testing.T) {
	t.Parallel()

	// A complete message that unpacks far enough to fail at a data field under the
	// wrong spec is the case the doctor hint is for. (A header-level failure is
	// reported as truncated/corrupt instead.)
	code, out, _ := runApp("", "validate", "../examples/spec87ascii/0800-network-echo.hex", "--spec", "spec87bcd-starter", "--no-color")
	if code != 1 {
		t.Fatalf("validate should fail: %d", code)
	}
	if !strings.Contains(out, "doctor") {
		t.Errorf("validate failure should hint at doctor\n%s", out)
	}
}

// TestDoctorAmbiguousListsAllTiedPresets guards that a tie does not present the
// default as the single answer: every preset tied at the best score is labeled
// "recommended" in the candidate list, and each gets a Confirm-with command.
func TestDoctorAmbiguousListsAllTiedPresets(t *testing.T) {
	t.Parallel()

	code, out, _ := runApp("", "doctor", "../examples/spec87ascii/0800-network-echo.hex", "--no-color")
	if code != 0 {
		t.Fatalf("doctor failed: %d\n%s", code, out)
	}
	// Both basei-starter and spec87ascii fit this message equally well.
	if strings.Count(out, "recommended") < 2 {
		t.Errorf("a tie should mark every tied preset recommended:\n%s", out)
	}
	for _, want := range []string{
		"iso8583tool view --spec basei-starter",
		"iso8583tool view --spec spec87ascii",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Confirm-with should list every tied preset; missing %q\n%s", want, out)
		}
	}
}

func TestDoctorConfirmHintIsShellSafe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src, err := os.ReadFile(example("0110-auth-response.hex"))
	if err != nil {
		t.Fatal(err)
	}

	// A path containing a space must be quoted.
	spaced := filepath.Join(dir, "with space.hex")
	if err := os.WriteFile(spaced, src, 0o600); err != nil {
		t.Fatal(err)
	}
	_, out, _ := runApp("", "doctor", spaced, "--no-color")
	if !strings.Contains(out, "'"+spaced+"'") {
		t.Errorf("a path with a space must be quoted in the confirm hint:\n%s", out)
	}

	// A "-"-prefixed filename must be placed after a "--" separator so it is not
	// parsed as a flag (tested at the command-builder level, since a real
	// "-"-prefixed relative path cannot be opened from the test working dir).
	if got := confirmCommand("-resp.hex", "basei-starter", ""); got != "iso8583tool view --spec basei-starter -- -resp.hex" {
		t.Errorf("confirmCommand for a dash path = %q", got)
	}
	if got := confirmCommand("/tmp/with space.hex", "basei-starter", " --encoding raw"); got != "iso8583tool view --spec basei-starter --encoding raw '/tmp/with space.hex'" {
		t.Errorf("confirmCommand for a spaced path = %q", got)
	}
}

// TestValidateHintBuiltinVsCustom checks that a failed unpack steers the user to
// doctor only for a built-in preset; for a custom --spec PATH (which doctor
// cannot detect) it points at the spec/capture instead.
func TestValidateHintBuiltinVsCustom(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A bitmap-composite custom spec and a message packed under a different custom
	// layout, so the message does not unpack under it.
	bitmapSpec := filepath.Join(dir, "f127-bitmap.json")
	if err := os.WriteFile(bitmapSpec, []byte(`{"name":"F127 bitmap","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"127":{"type":"Composite","length":255,"description":"Private use field","prefix":"ASCII.LL","bitmap":{"type":"Bitmap","length":8,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed","disableAutoExpand":true},"subfields":{"1":{"type":"String","length":2,"description":"A","enc":"ASCII","prefix":"ASCII.Fixed"},"2":{"type":"String","length":2,"description":"B","enc":"ASCII","prefix":"ASCII.Fixed"}}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	posSpec := filepath.Join(dir, "f48-pos.json")
	if err := os.WriteFile(posSpec, []byte(`{"name":"F48 positional","fields":{"0":{"type":"String","length":4,"description":"MTI","enc":"ASCII","prefix":"ASCII.Fixed"},"1":{"type":"Bitmap","length":16,"description":"Bitmap","enc":"HexToASCII","prefix":"Hex.Fixed"},"11":{"type":"String","length":6,"description":"STAN","enc":"ASCII","prefix":"ASCII.Fixed"},"48":{"type":"Composite","length":999,"description":"Private Data","prefix":"ASCII.LLL","tag":{"sort":"StringsByInt"},"subfields":{"1":{"type":"String","length":3,"description":"A","enc":"ASCII","prefix":"ASCII.Fixed"},"2":{"type":"String","length":2,"description":"B","enc":"ASCII","prefix":"ASCII.Fixed"}}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	hexOut := filepath.Join(dir, "msg.hex")
	if code, _, errOut := runApp(`{"mti":"0100","fields":{"11":"123456","127.1":"AA","127.2":"BB"}}`, "convert", "--to", "hex", "--spec", bitmapSpec, "--output", hexOut); code != 0 {
		t.Fatalf("convert failed: %s", errOut)
	}

	// Custom spec mismatch: must NOT mention doctor.
	_, out, _ := runApp("", "validate", hexOut, "--spec", posSpec, "--encoding", "hex", "--no-color")
	if strings.Contains(out, "doctor") {
		t.Errorf("a custom-spec failure must not point at doctor:\n%s", out)
	}
	if !strings.Contains(out, "spec file") {
		t.Errorf("a custom-spec failure should point at the spec/capture:\n%s", out)
	}

	// Built-in spec mismatch: should mention doctor.
	_, out2, _ := runApp("", "validate", example("0110-auth-response.hex"), "--spec", "spec87bcd-starter", "--no-color")
	if !strings.Contains(out2, "doctor") {
		t.Errorf("a built-in-preset failure should point at doctor:\n%s", out2)
	}
}

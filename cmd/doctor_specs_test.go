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

	code, out, _ := runApp("", "validate", "--raw", "01007220", "--no-color")
	if code != 1 {
		t.Fatalf("validate should fail: %d", code)
	}
	if !strings.Contains(out, "doctor") {
		t.Errorf("validate failure should hint at doctor\n%s", out)
	}
}

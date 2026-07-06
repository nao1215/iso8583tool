package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestConvertErrorBranches drives the reachable early-return branches of
// runConvert (direction, wrong-input-shape, parse, load, read, and arg-count
// errors), each of which prints to stderr and exits 1.
func TestConvertErrorBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{"bad direction", []string{"convert", "--to", "bogus", "--raw", `{"mti":"0100"}`}, "", "unsupported --to"},
		{"to-hex non-json", []string{"convert", "--to", "hex", "--raw", "not-json-at-all"}, "", "not JSON"},
		{"to-json is-json", []string{"convert", "--to", "json", "--raw", `{"mti":"0100"}`}, "", "already looks like a JSON document"},
		{"to-hex malformed json", []string{"convert", "--to", "hex", "--raw", "{bad json"}, "", ""},
		{"missing spec file", []string{"convert", "--spec", "/no/such/spec.json", "--raw", `{"mti":"0100"}`, "--to", "hex"}, "", ""},
		{"missing input file", []string{"convert", "/no/such/file.hex"}, "", ""},
		{"too many args", []string{"convert", "a", "b"}, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			code, _, errOut := runApp(c.stdin, c.args...)
			if code != 1 {
				t.Fatalf("%s: code=%d, want 1 (err=%q)", c.name, code, errOut)
			}
			if c.want != "" && !strings.Contains(errOut, c.want) {
				t.Errorf("%s: stderr=%q, want to contain %q", c.name, errOut, c.want)
			}
		})
	}
}

// TestValidateErrorBranches drives runValidate's reachable error branches.
func TestValidateErrorBranches(t *testing.T) {
	t.Parallel()

	good := example("0110-auth-response.hex")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"bad color", []string{"validate", good, "--color", "banana"}, "invalid --color"},
		{"unsupported format", []string{"validate", good, "--format", "bogus"}, "unsupported format"},
		{"missing spec", []string{"validate", good, "--spec", "/no/such/spec.json"}, ""},
		{"missing input", []string{"validate", "/no/such/file.hex"}, ""},
		{"too many args", []string{"validate", "a", "b"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			code, _, errOut := runApp("", c.args...)
			if code != 1 {
				t.Fatalf("%s: code=%d, want 1 (err=%q)", c.name, code, errOut)
			}
			if c.want != "" && !strings.Contains(errOut, c.want) {
				t.Errorf("%s: stderr=%q, want to contain %q", c.name, errOut, c.want)
			}
		})
	}
}

// TestRedactViewDiffErrorBranches drives the shared error paths of runRedact,
// runView, and runDiff (bad color, missing spec, missing input, too many args).
func TestRedactViewDiffErrorBranches(t *testing.T) {
	t.Parallel()

	good := example("0110-auth-response.hex")
	cases := [][]string{
		{"redact", good, "--color", "banana"},
		{"redact", "/no/such/file.hex"},
		{"redact", "a", "b"},
		{"view", good, "--spec", "/no/such/spec.json"},
		{"view", "/no/such/file.hex"},
		{"diff", good},                      // diff needs two inputs
		{"diff", good, "/no/such/file.hex"}, // second input missing
		{"diff", "/no/such/a.hex", good},    // first input missing
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			t.Parallel()
			code, _, errOut := runApp("", args...)
			if code != 1 {
				t.Fatalf("%v: code=%d, want 1 (err=%q)", args, code, errOut)
			}
		})
	}
}

// TestSampleFormatError drives renderSample's unsupported-format branch and the
// too-many-args branch of runSample.
func TestSampleFormatError(t *testing.T) {
	t.Parallel()

	if code, _, errOut := runApp("", "sample", "0100-auth-request", "--format", "bogus"); code != 1 || !strings.Contains(errOut, "unsupported sample format") {
		t.Fatalf("sample bad format: code=%d err=%q", code, errOut)
	}
	if code, _, _ := runApp("", "sample", "a", "b"); code != 1 {
		t.Fatalf("sample too many args: code=%d", code)
	}
}

// TestSampleHexToFile covers the renderSample hex branch together with the
// --output write path.
func TestSampleHexToFile(t *testing.T) {
	t.Parallel()

	out := filepath.Join(t.TempDir(), "sample.hex")
	code, stdout, errOut := runApp("", "sample", "0100-auth-request", "--format", "hex", "--output", out)
	if code != 0 {
		t.Fatalf("sample hex to file: code=%d err=%q", code, errOut)
	}
	if !strings.Contains(stdout, "Wrote sample") {
		t.Errorf("expected a write confirmation, got %q", stdout)
	}
}

// TestDoctorErrorBranches drives runDoctor's reachable error paths.
func TestDoctorErrorBranches(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		{"doctor", "/no/such/file.hex"},
		{"doctor", "a", "b"},
		{"doctor", example("0110-auth-response.hex"), "--format", "bogus"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			t.Parallel()
			code, _, errOut := runApp("", args...)
			if code != 1 {
				t.Fatalf("%v: code=%d, want 1 (err=%q)", args, code, errOut)
			}
		})
	}
}

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/render"
	"github.com/nao1215/iso8583tool/internal/service"
)

func TestStrategyColor(t *testing.T) {
	t.Parallel()
	pal := render.NewPalette(true)
	for _, strategy := range []string{"tlv", "opaque", "positional", "bitmap"} {
		got := strategyColor(pal, strategy)
		if !strings.Contains(got, strategy) {
			t.Errorf("strategyColor(%q) = %q, want to contain the strategy", strategy, got)
		}
		// A recognized strategy is colorized, so it differs from the raw string.
		if got == strategy {
			t.Errorf("strategyColor(%q) was not colorized", strategy)
		}
	}
	// Unknown strategy is returned unchanged.
	if got := strategyColor(pal, "mystery"); got != "mystery" {
		t.Errorf("strategyColor(unknown) = %q, want unchanged", got)
	}
}

func TestSeverityColor(t *testing.T) {
	t.Parallel()
	pal := render.NewPalette(true)
	for _, sev := range []string{"error", "warning"} {
		got := severityColor(pal, sev)
		if got == sev {
			t.Errorf("severityColor(%q) was not colorized", sev)
		}
	}
	// The default (info/other) branch dims the value.
	if got := severityColor(pal, "info"); !strings.Contains(got, "info") {
		t.Errorf("severityColor(info) = %q, want to contain info", got)
	}
}

func TestAmbiguityHint(t *testing.T) {
	t.Parallel()
	hint := ambiguityHint([]string{basei.StarterPreset, basei.Spec87ASCIIPreset})
	if !strings.Contains(hint, "Field 55") {
		t.Errorf("ambiguityHint tie = %q, want the Field 55 explanation", hint)
	}
	if got := ambiguityHint([]string{basei.StarterPreset}); got != "" {
		t.Errorf("ambiguityHint(single) = %q, want empty", got)
	}
}

func TestRecommendedSet(t *testing.T) {
	t.Parallel()
	// Explicit recommendations become the set.
	set := recommendedSet(service.SpecDiagnosis{Recommendations: []string{"a", "b"}})
	if !set["a"] || !set["b"] || len(set) != 2 {
		t.Errorf("recommendedSet(list) = %v", set)
	}
	// Falls back to the single Recommended when no list is present.
	fallback := recommendedSet(service.SpecDiagnosis{Recommended: "solo"})
	if !fallback["solo"] || len(fallback) != 1 {
		t.Errorf("recommendedSet(fallback) = %v", fallback)
	}
	// Neither set -> empty.
	if got := recommendedSet(service.SpecDiagnosis{}); len(got) != 0 {
		t.Errorf("recommendedSet(empty) = %v", got)
	}
}

func TestShellQuoteArg(t *testing.T) {
	t.Parallel()
	if got := shellQuoteArg("message.bin"); got != "message.bin" {
		t.Errorf("shellQuoteArg(safe) = %q, want unchanged", got)
	}
	if got := shellQuoteArg(""); got != "''" {
		t.Errorf("shellQuoteArg(empty) = %q, want ''", got)
	}
	if got := shellQuoteArg("a b"); got != "'a b'" {
		t.Errorf("shellQuoteArg(space) = %q, want quoted", got)
	}
	if got := shellQuoteArg("it's"); got != `'it'\''s'` {
		t.Errorf("shellQuoteArg(quote) = %q", got)
	}
}

func TestIsPortableArgChar(t *testing.T) {
	t.Parallel()
	for _, r := range []rune{'a', 'Z', '5', '_', '-', '.', '/', '+', '='} {
		if !isPortableArgChar(r) {
			t.Errorf("isPortableArgChar(%q) = false, want true", r)
		}
	}
	for _, r := range []rune{' ', '\'', '$', '*', '\n'} {
		if isPortableArgChar(r) {
			t.Errorf("isPortableArgChar(%q) = true, want false", r)
		}
	}
}

func TestConfirmCommand(t *testing.T) {
	t.Parallel()
	// Empty / "-" target yields the MESSAGE placeholder form.
	if got := confirmCommand("", "basei-starter", " --enc hex"); !strings.Contains(got, "MESSAGE") {
		t.Errorf("confirmCommand(empty) = %q", got)
	}
	if got := confirmCommand("-", "basei-starter", ""); !strings.Contains(got, "MESSAGE") {
		t.Errorf("confirmCommand(dash) = %q", got)
	}
	// A dash-prefixed filename is placed after a "--" separator.
	got := confirmCommand("-weird.bin", "basei-starter", "")
	if !strings.Contains(got, "-- ") {
		t.Errorf("confirmCommand(dash-prefixed) = %q, want a -- separator", got)
	}
	// A plain target is shell-quoted and appended.
	if got := confirmCommand("msg.bin", "spec87ascii", ""); !strings.Contains(got, "msg.bin") {
		t.Errorf("confirmCommand(plain) = %q", got)
	}
}

func TestHelpRequested(t *testing.T) {
	t.Parallel()
	if !helpRequested([]string{"-h"}) {
		t.Error("helpRequested(-h) = false")
	}
	if !helpRequested([]string{"view", "--help"}) {
		t.Error("helpRequested(--help) = false")
	}
	if helpRequested([]string{"view", "msg.bin"}) {
		t.Error("helpRequested(no help) = true")
	}
	// A "--" terminator stops help detection.
	if helpRequested([]string{"--", "-h"}) {
		t.Error("helpRequested(-- -h) = true, want false")
	}
}

func TestParseArgs(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	// Success path.
	fs := newFlagSet("test", &stderr)
	fs.String("format", "text", "")
	if code, ok := parseArgs(fs, []string{"--format", "json"}); code != 0 || !ok {
		t.Errorf("parseArgs(valid) = %d,%v", code, ok)
	}

	// Unknown flag -> error code 1.
	fs2 := newFlagSet("test", &stderr)
	if code, ok := parseArgs(fs2, []string{"--nope"}); code != 1 || ok {
		t.Errorf("parseArgs(bad) = %d,%v", code, ok)
	}

	// Explicit help (-h) -> ErrHelp branch: code 0, ok false.
	fs3 := newFlagSet("test", &stderr)
	if code, ok := parseArgs(fs3, []string{"-h"}); code != 0 || ok {
		t.Errorf("parseArgs(help) = %d,%v", code, ok)
	}
}

// TestResolveVersion mutates the package-level Version, so it deliberately does
// not run in parallel with the tests that invoke the `version` command.
//
//nolint:paralleltest // mutates global Version; must stay serial
func TestResolveVersion(t *testing.T) {
	// With Version overridden away from the dev sentinel it is returned as-is.
	saved := Version
	defer func() { Version = saved }()
	Version = "v9.9.9"
	if got := resolveVersion(); got != "v9.9.9" {
		t.Errorf("resolveVersion(set) = %q, want v9.9.9", got)
	}
	// The dev sentinel falls back to build info (or the sentinel in tests).
	Version = devVersion
	if got := resolveVersion(); got == "" {
		t.Error("resolveVersion(dev) returned empty")
	}
}

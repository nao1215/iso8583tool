package cmd

import (
	"strings"
	"testing"
)

// TestSubcommandsNameTheUnexpectedArgument is the regression for #58: a
// subcommand handed more positional arguments than it takes exited 1 with the
// usage block and nothing else, so the reader had to count the arguments in the
// Usage line themselves to work out what was wrong. Every other refusal in this
// tool names what it saw -- `unknown sample "x"`, `mti must be exactly 4
// digits, got "010"` -- and this is the one family that did not.
//
// The whole family is covered rather than the three commands the issue happens
// to name: the guards are a copy of each other, so a message added to one of
// them proves nothing about the rest. `specs` takes no positional argument at
// all, which is the boundary on the other side of the same rule.
func TestSubcommandsNameTheUnexpectedArgument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		unexpect string
	}{
		{"view takes one message", []string{"view", "a.hex", "b.hex"}, "b.hex"},
		{"redact takes one message", []string{"redact", "a.hex", "b.hex"}, "b.hex"},
		{"convert takes one message", []string{"convert", "a.hex", "b.hex"}, "b.hex"},
		{"validate takes one message", []string{"validate", "a.hex", "b.hex"}, "b.hex"},
		{"doctor takes one message", []string{"doctor", "a.hex", "b.hex"}, "b.hex"},
		{"sample takes one name", []string{"sample", "auth", "extra"}, "extra"},
		{"specs takes none", []string{"specs", "extra"}, "extra"},
		{"send takes an address and one message", []string{"send", "127.0.0.1:1", "a.hex", "b.hex"}, "b.hex"},
		{"diff takes exactly two messages", []string{"diff", "a.hex", "b.hex", "c.hex"}, "c.hex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code, _, stderr := runApp("", tt.args...)
			if code != 1 {
				t.Fatalf("%v: exit %d, want 1", tt.args, code)
			}
			if !strings.Contains(stderr, "too many arguments") {
				t.Errorf("%v: stderr does not say what was wrong:\n%s", tt.args, stderr)
			}
			if !strings.Contains(stderr, `"`+tt.unexpect+`"`) {
				t.Errorf("%v: stderr does not name the unexpected argument %q:\n%s", tt.args, tt.unexpect, stderr)
			}
		})
	}
}

// TestDiffNamesTooFewArguments is the other half of diff's arity rule. diff is
// the only subcommand that requires two positional arguments, so it is the only
// one where "not enough" and "too many" are different mistakes, and the single
// `NArg() != 2` guard answered both with the same silent usage block.
func TestDiffNamesTooFewArguments(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"diff"}, {"diff", "only.hex"}} {
		code, _, stderr := runApp("", args...)
		if code != 1 {
			t.Fatalf("%v: exit %d, want 1", args, code)
		}
		if !strings.Contains(stderr, "diff takes two messages") {
			t.Errorf("%v: stderr does not say how many messages diff takes:\n%s", args, stderr)
		}
	}
}

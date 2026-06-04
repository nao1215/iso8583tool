package service

import (
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messagespec"
)

func diffSpec(t *testing.T) *messagespec.Spec {
	t.Helper()
	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	return spec
}

func TestDiffMessagesChangedAddedRemoved(t *testing.T) {
	t.Parallel()
	spec := diffSpec(t)

	before := basei.AuthRequest()
	before.Fields["4"] = "000000001000"
	before.BinaryFields["55.9F02"] = "000000001000"

	after := basei.AuthRequest()
	after.Fields["4"] = "000000005000"             // changed
	after.BinaryFields["55.9F02"] = "000000005000" // changed
	delete(after.Fields, "62")                     // removed
	after.Fields["32"] = "12345"                   // added (LLVAR)

	beforeRaw, err := WriteMessage(before, spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack before: %v", err)
	}
	afterRaw, err := WriteMessage(after, spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack after: %v", err)
	}

	result, err := DiffMessages(spec.MessageSpec, beforeRaw.Raw, afterRaw.Raw, nil)
	if err != nil {
		t.Fatalf("DiffMessages: %v", err)
	}

	got := map[string]DiffEntry{}
	for _, c := range result.Changes {
		got[c.Path] = c
	}

	if c := got["4"]; c.Kind != DiffChanged || c.Before != "000000001000" || c.After != "000000005000" {
		t.Fatalf("field 4 diff = %#v", c)
	}
	if c := got["55.9F02"]; c.Kind != DiffChanged {
		t.Fatalf("field 55.9F02 should be changed: %#v", c)
	}
	if c := got["62"]; c.Kind != DiffRemoved || c.Before == "" {
		t.Fatalf("field 62 should be removed: %#v", c)
	}
	if c := got["32"]; c.Kind != DiffAdded || c.After == "" {
		t.Fatalf("field 32 should be added: %#v", c)
	}
}

func TestDiffMessagesIdentical(t *testing.T) {
	t.Parallel()
	spec := diffSpec(t)

	raw, err := WriteMessage(basei.AuthRequest(), spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	result, err := DiffMessages(spec.MessageSpec, raw.Raw, raw.Raw, nil)
	if err != nil {
		t.Fatalf("DiffMessages: %v", err)
	}
	if len(result.Changes) != 0 {
		t.Fatalf("identical messages should have no changes, got %#v", result.Changes)
	}
}

func TestDiffDeterministicOrderAndFilter(t *testing.T) {
	t.Parallel()
	spec := diffSpec(t)

	before := basei.AuthRequest()
	before.Fields["4"] = "000000001000"
	before.BinaryFields["55.9F02"] = "000000001000"

	after := basei.AuthRequest()
	after.Fields["4"] = "000000005000"
	after.BinaryFields["55.9F02"] = "000000005000"
	after.BinaryFields["55.9F36"] = "0099"

	beforeRaw, _ := WriteMessage(before, spec.MessageSpec)
	afterRaw, _ := WriteMessage(after, spec.MessageSpec)

	// Unfiltered ordering is deterministic and path-sorted (4 before 55.*).
	all, err := DiffMessages(spec.MessageSpec, beforeRaw.Raw, afterRaw.Raw, nil)
	if err != nil {
		t.Fatalf("DiffMessages: %v", err)
	}
	if len(all.Changes) < 3 || all.Changes[0].Path != "4" {
		t.Fatalf("unexpected ordering: %#v", all.Changes)
	}

	// --filter 55 keeps only the EMV subtag changes.
	filtered, err := DiffMessages(spec.MessageSpec, beforeRaw.Raw, afterRaw.Raw, []string{"55"})
	if err != nil {
		t.Fatalf("DiffMessages filtered: %v", err)
	}
	for _, c := range filtered.Changes {
		if c.Path != "55.9F02" && c.Path != "55.9F36" {
			t.Fatalf("filter 55 leaked path %q", c.Path)
		}
	}
	if len(filtered.Changes) != 2 {
		t.Fatalf("filter 55 expected 2 changes, got %#v", filtered.Changes)
	}
}

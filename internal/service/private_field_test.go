package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messageio"
	"github.com/nao1215/iso8583tool/internal/messagespec"
	"github.com/nao1215/iso8583tool/internal/render"
)

const embeddedPAN = "4111111111111111"

func privateFieldDoc(pan string) messageio.Document {
	return messageio.Document{
		MTI:    "0110",
		Fields: map[string]string{"11": "123456", "39": "00", "63": "PAN=" + pan},
	}
}

func privateSpec(t *testing.T) *messagespec.Spec {
	t.Helper()
	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	return spec
}

// TestRedactMasksEmbeddedPANInPrivateField is reproduction case 1: redact must
// not leak a PAN embedded in a free-form private field (F63).
func TestRedactMasksEmbeddedPANInPrivateField(t *testing.T) {
	t.Parallel()
	spec := privateSpec(t)

	raw, err := WriteMessage(privateFieldDoc(embeddedPAN), spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	red, paths, err := RedactMessage(spec.MessageSpec, raw.Raw)
	if err != nil {
		t.Fatalf("RedactMessage: %v", err)
	}
	if strings.Contains(red.Fields["63"], embeddedPAN) {
		t.Fatalf("redact leaked the PAN in F63: %q", red.Fields["63"])
	}
	if !contains(paths, "63") {
		t.Fatalf("redact should report F63 as redacted, got %v", paths)
	}
}

// TestViewDefaultMasksEmbeddedPAN is reproduction case 2: the default view (json
// and describe) must not leak a PAN embedded in F63.
func TestViewDefaultMasksEmbeddedPAN(t *testing.T) {
	t.Parallel()
	spec := privateSpec(t)

	raw, err := WriteMessage(privateFieldDoc(embeddedPAN), spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	for _, format := range []string{"json", "describe"} {
		res, err := ViewMessage(raw.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), format, nil, render.NewPalette(false), false)
		if err != nil {
			t.Fatalf("ViewMessage(%s): %v", format, err)
		}
		if strings.Contains(res.Body, embeddedPAN) {
			t.Fatalf("default view %s leaked the embedded PAN:\n%s", format, res.Body)
		}
	}
	// A filtered view of F63 must mask it too.
	res, err := ViewMessage(raw.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "describe", []string{"63"}, render.NewPalette(false), false)
	if err != nil {
		t.Fatalf("filtered view: %v", err)
	}
	if strings.Contains(res.Body, embeddedPAN) {
		t.Fatalf("filtered view leaked the embedded PAN:\n%s", res.Body)
	}
}

// TestViewUnsafeShowsRaw verifies the explicit opt-in reveals the raw value.
func TestViewUnsafeShowsRaw(t *testing.T) {
	t.Parallel()
	spec := privateSpec(t)

	raw, err := WriteMessage(privateFieldDoc(embeddedPAN), spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	for _, format := range []string{"json", "describe"} {
		res, err := ViewMessage(raw.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), format, nil, render.NewPalette(false), true)
		if err != nil {
			t.Fatalf("ViewMessage(%s, unsafe): %v", format, err)
		}
		if !strings.Contains(res.Body, embeddedPAN) {
			t.Fatalf("unsafe view %s should show the raw PAN:\n%s", format, res.Body)
		}
	}
}

// TestDiffMasksEmbeddedPANInPrivateField is reproduction case 3: the default
// diff must not leak a PAN embedded in F63; --unsafe reveals it.
func TestDiffMasksEmbeddedPANInPrivateField(t *testing.T) {
	t.Parallel()
	spec := privateSpec(t)

	rawA, err := WriteMessage(privateFieldDoc("4111111111111111"), spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack a: %v", err)
	}
	rawB, err := WriteMessage(privateFieldDoc("4222222222222222"), spec.MessageSpec)
	if err != nil {
		t.Fatalf("pack b: %v", err)
	}

	safe, err := DiffMessages(spec.MessageSpec, rawA.Raw, rawB.Raw, nil, false)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	for _, c := range safe.Changes {
		if strings.Contains(c.Before, "4111111111111111") || strings.Contains(c.After, "4222222222222222") {
			t.Fatalf("default diff leaked an embedded PAN: %#v", c)
		}
	}

	unsafe, err := DiffMessages(spec.MessageSpec, rawA.Raw, rawB.Raw, nil, true)
	if err != nil {
		t.Fatalf("diff unsafe: %v", err)
	}
	leaked := false
	for _, c := range unsafe.Changes {
		if strings.Contains(c.After, "4222222222222222") {
			leaked = true
		}
	}
	if !leaked {
		t.Fatalf("--unsafe diff should reveal the raw PAN, got %#v", unsafe.Changes)
	}
}

package service

import (
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messagespec"
	"github.com/nao1215/iso8583tool/internal/render"
)

func TestWriteValidateAndView(t *testing.T) {
	t.Parallel()

	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load returned error: %v", err)
	}

	doc := basei.AuthRequest()
	writeResult, err := WriteMessage(doc, spec.MessageSpec)
	if err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	report := ValidateMessage(writeResult.Raw, spec.MessageSpec, spec.Label, basei.DefaultExtensionCatalog())
	if report.HasErrors() {
		t.Fatalf("ValidateMessage returned errors: %#v", report.Issues)
	}
	if report.MTI != "0100" {
		t.Fatalf("report.MTI = %q, want 0100", report.MTI)
	}
	if len(report.Extensions) < 3 {
		t.Fatal("expected extension notices for fields 48, 55, and 62")
	}

	viewResult, err := ViewMessage(writeResult.Raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false))
	if err != nil {
		t.Fatalf("ViewMessage returned error: %v", err)
	}
	if viewResult.Body == "" {
		t.Fatal("expected describe output")
	}
	if len(viewResult.UnknownTags) != 0 {
		t.Fatalf("expected no unknown tags, got %#v", viewResult.UnknownTags)
	}
}

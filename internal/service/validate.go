package service

import (
	"fmt"

	"github.com/moov-io/iso8583"

	"github.com/nao1215/iso8583tool/internal/annotate"
	"github.com/nao1215/iso8583tool/internal/basei"
)

type ValidationIssue struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type ExtensionNotice struct {
	Field    int    `json:"field"`
	Name     string `json:"name"`
	Strategy string `json:"strategy"`
	Note     string `json:"note"`
}

type ValidationReport struct {
	Valid          bool              `json:"valid"`
	Spec           string            `json:"spec"`
	MTI            string            `json:"mti,omitempty"`
	MTIDescription string            `json:"mti_description,omitempty"`
	Summary        string            `json:"summary,omitempty"`
	Issues         []ValidationIssue `json:"issues,omitempty"`
	Extensions     []ExtensionNotice `json:"extensions,omitempty"`
	UnknownTags    []UnknownTag      `json:"unknown_tags,omitempty"`
	Decoded        []DecodedField    `json:"decoded,omitempty"`
}

func (r ValidationReport) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

func ValidateMessage(raw []byte, spec *iso8583.MessageSpec, specLabel string, catalog basei.ExtensionCatalog) ValidationReport {
	report := ValidationReport{
		Spec: specLabel,
	}

	msg := iso8583.NewMessage(spec)
	if err := msg.Unpack(raw); err != nil {
		diag := diagnoseUnpack(err, raw)
		path := diag.Path
		if path == "" {
			path = "message"
		}
		report.Issues = append(report.Issues, ValidationIssue{
			Severity: "error",
			Path:     path,
			Message:  fmt.Sprintf("%s (input was %d bytes)", diag.Cause, diag.Bytes),
		})
		report.Valid = false
		return report
	}

	mti, err := msg.GetMTI()
	if err != nil {
		report.Issues = append(report.Issues, ValidationIssue{
			Severity: "error",
			Path:     "0",
			Message:  err.Error(),
		})
	} else {
		report.MTI = mti
		if mti == "" {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Path:     "0",
				Message:  "missing MTI",
			})
		} else {
			report.MTIDescription = annotate.MTI(mti)
		}
	}

	report.Decoded = DecodeFields(msg)
	report.Summary = Summarize(msg)

	// An extension field's strategy (opaque, tlv, positional, bitmap) is a
	// configured presentation choice, not a defect. It is reported under
	// "Extension Field Strategy" only. Issues are reserved for real problems
	// such as unknown TLV tags or fields that fail to unpack.
	for _, ext := range activeExtensions(msg.GetFields(), catalog) {
		report.Extensions = append(report.Extensions, ExtensionNotice{
			Field:    ext.ID,
			Name:     ext.Name,
			Strategy: string(ext.Strategy),
			Note:     extensionNote(ext),
		})
	}

	report.UnknownTags = collectUnknownTags(msg)
	for _, unknown := range report.UnknownTags {
		report.Issues = append(report.Issues, ValidationIssue{
			Severity: "warning",
			Path:     unknown.Path,
			Message:  "unknown TLV tag preserved for round-trip safety",
		})
	}

	report.Valid = !report.HasErrors()
	return report
}

func extensionNote(ext basei.ExtensionField) string {
	switch ext.Strategy {
	case basei.StrategyTLV:
		return "Prefer composite/TLV decoding and preserve unknown tags to keep round-trip safety."
	case basei.StrategyPositional:
		return "Best edited as dot-path subfields after the private layout is stable."
	case basei.StrategyBitmap:
		return "Reserve a dedicated nested-bitmap overlay instead of flattening directly into top-level fields."
	default:
		return "Keep raw until the partner-specific payload shape is stable enough to model."
	}
}

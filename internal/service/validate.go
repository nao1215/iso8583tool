package service

import (
	"fmt"
	"strconv"
	"strings"

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

// ValidateMessage checks that a message unpacks and reports unknown TLV tags.
// When strict is set it additionally applies best-effort, message-class-aware
// BASE I semantic checks (required and recommended fields per MTI class). Strict
// mode is a heuristic aid, not a substitute for full network certification.
func ValidateMessage(raw []byte, spec *iso8583.MessageSpec, specLabel string, catalog basei.ExtensionCatalog, strict bool) ValidationReport {
	report := ValidationReport{
		Spec: specLabel,
	}

	msg := iso8583.NewMessage(spec)
	if err := safeUnpack(msg, raw); err != nil {
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

	// Mask the bytes of unknown tags: they can hold cardholder data and the
	// report only needs to flag the tag path, not its contents.
	report.UnknownTags = maskUnknownTagValues(collectUnknownTags(msg))
	for _, unknown := range report.UnknownTags {
		report.Issues = append(report.Issues, ValidationIssue{
			Severity: "warning",
			Path:     unknown.Path,
			Message:  "unknown TLV tag preserved for round-trip safety",
		})
	}

	if strict {
		report.Issues = append(report.Issues, strictSemanticIssues(msg, report.MTI)...)
	}

	report.Valid = !report.HasErrors()
	return report
}

// strictSemanticIssues applies message-class-aware BASE I checks. It is a
// best-effort heuristic: it covers the common required/recommended fields for
// authorization, financial, reversal, and network-management messages, keyed by
// the MTI message class and function. It does not model every conditional rule
// or partner-specific overlay.
func strictSemanticIssues(msg *iso8583.Message, mti string) []ValidationIssue {
	var issues []ValidationIssue
	add := func(severity, path, message string) {
		issues = append(issues, ValidationIssue{Severity: severity, Path: path, Message: message})
	}

	if len(mti) != 4 {
		add("error", "0", "strict: MTI must be 4 digits to classify the message")
		return issues
	}
	for _, r := range mti {
		if r < '0' || r > '9' {
			add("error", "0", "strict: MTI must be numeric")
			return issues
		}
	}

	fields := msg.GetFields()
	has := func(id int) bool { _, ok := fields[id]; return ok }
	hasAny := func(ids ...int) bool {
		for _, id := range ids {
			if has(id) {
				return true
			}
		}
		return false
	}
	require := func(id int, context string) {
		if !has(id) {
			add("error", strconv.Itoa(id), "strict: required for "+context)
		}
	}
	recommend := func(id int, context string) {
		if !has(id) {
			add("warning", strconv.Itoa(id), "strict: recommended for "+context)
		}
	}

	class, function := mti[1], mti[2]
	isRequest := function == '0'
	isResponse := function == '1' || function == '3'
	isAdvice := function == '2'

	// The system trace audit number (field 11) ties a message to its pair.
	require(11, "every BASE I message")

	switch class {
	case '1', '2': // authorization / financial
		switch {
		case isRequest:
			require(3, "an authorization/financial request (processing code)")
			require(4, "an authorization/financial request (amount)")
			require(7, "an authorization/financial request (transmission date/time)")
			if !hasAny(2, 35, 45) {
				add("error", "2", "strict: an authorization/financial request needs a PAN source (field 2, 35, or 45)")
			}
			recommend(37, "card messages (retrieval reference number)")
		case isResponse:
			require(39, "an authorization/financial response (response code)")
			if rc, err := msg.GetString(39); err == nil && isApprovalCode(rc) && !has(38) {
				add("warning", "38", "strict: an approved response (field 39="+strings.TrimSpace(rc)+") usually carries an authorization identification (field 38)")
			}
			recommend(37, "card messages (retrieval reference number)")
		}
	case '4': // reversal
		switch {
		case isRequest || isAdvice:
			require(4, "a reversal (amount)")
			require(7, "a reversal (transmission date/time)")
			require(90, "a reversal (original data elements)")
		case isResponse:
			require(39, "a reversal response (response code)")
		}
	case '8': // network management
		switch {
		case isRequest:
			require(70, "a network-management request (network management code)")
		case isResponse:
			require(39, "a network-management response (response code)")
		}
	}

	return issues
}

// isApprovalCode reports whether a BASE I response code (field 39) is an
// approval, for which an authorization identification (field 38) is expected.
func isApprovalCode(rc string) bool {
	switch strings.TrimSpace(rc) {
	case "00", "000", "0000", "10", "11":
		return true
	default:
		return false
	}
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

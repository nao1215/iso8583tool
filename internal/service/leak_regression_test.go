package service

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/config"
	"github.com/nao1215/iso8583tool/internal/messagespec"
	"github.com/nao1215/iso8583tool/internal/render"
)

func regressionSpec(t *testing.T) *messagespec.Spec {
	t.Helper()
	spec, err := messagespec.Load(".", config.Default())
	if err != nil {
		t.Fatalf("messagespec.Load: %v", err)
	}
	return spec
}

// TestPINFieldIsHexAndMasked is a regression for a bug found by FuzzConvertRoundTrip:
// field 52 (PIN, a binary-encoded primitive) was emitted into the text `fields`
// map, which put raw control bytes in the JSON and — worse — meant redact and
// view never masked the PIN block (sensitive authentication data). It must now
// be hex in binary_fields and masked everywhere it is displayed.
func TestPINFieldIsHexAndMasked(t *testing.T) {
	t.Parallel()
	spec := regressionSpec(t)

	const pin = "0123456789ABCDEF"
	msg := iso8583.NewMessage(spec.MessageSpec)
	msg.MTI("0100")
	if err := msg.Field(2, "4111111111111111"); err != nil {
		t.Fatalf("set F2: %v", err)
	}
	if err := msg.Field(11, "123456"); err != nil {
		t.Fatalf("set F11: %v", err)
	}
	if err := msg.Field(52, pin); err != nil {
		t.Fatalf("set F52: %v", err)
	}
	raw, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	doc, err := MessageToDocument(spec.MessageSpec, raw)
	if err != nil {
		t.Fatalf("MessageToDocument: %v", err)
	}
	if _, inText := doc.Fields["52"]; inText {
		t.Fatalf("F52 must not be a text field (raw bytes leak into JSON): %#v", doc.Fields)
	}
	if _, inBinary := doc.BinaryFields["52"]; !inBinary {
		t.Fatalf("F52 should be emitted as a binary (hex) field, got %#v", doc.BinaryFields)
	}

	// redact must mask the PIN.
	red, _, err := RedactMessage(spec.MessageSpec, raw)
	if err != nil {
		t.Fatalf("RedactMessage: %v", err)
	}
	if v := red.BinaryFields["52"]; strings.Trim(v, "*") != "" {
		t.Fatalf("redact left the PIN unmasked: %q", v)
	}

	// view (json and describe) must never print the PIN.
	for _, format := range []string{"json", "describe"} {
		res, err := ViewMessage(raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), format, nil, render.NewPalette(false), false)
		if err != nil {
			t.Fatalf("ViewMessage(%s): %v", format, err)
		}
		if strings.Contains(res.Body, pin) {
			t.Fatalf("view %s leaked the PIN block:\n%s", format, res.Body)
		}
	}
}

// TestViewDescribeMasksTrackData is a regression for a bug found by
// FuzzViewNeverLeaksPAN: the text/describe view relied on moov's Track filters,
// which pass a value through unchanged when it is not parseable track data (and
// in the basei-starter spec the track fields are plain strings, so they never
// parse). Full track data — including expiry and discretionary data — leaked.
func TestViewDescribeMasksTrackData(t *testing.T) {
	t.Parallel()
	spec := regressionSpec(t)

	const track2 = "4111111111111111D29122011234567890" // PAN + expiry + discretionary
	const track1 = "A0000000000000000000000000000000"   // not well-formed track1

	msg := iso8583.NewMessage(spec.MessageSpec)
	msg.MTI("0100")
	if err := msg.Field(11, "123456"); err != nil {
		t.Fatalf("set F11: %v", err)
	}
	if err := msg.Field(35, track2); err != nil {
		t.Fatalf("set F35: %v", err)
	}
	if err := msg.Field(45, track1); err != nil {
		t.Fatalf("set F45: %v", err)
	}
	raw, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	res, err := ViewMessage(raw, spec.MessageSpec, basei.DefaultExtensionCatalog(), "describe", nil, render.NewPalette(false), false)
	if err != nil {
		t.Fatalf("ViewMessage: %v", err)
	}
	for _, secret := range []string{track2, track1} {
		if strings.Contains(res.Body, secret) {
			t.Fatalf("view describe leaked track data %q:\n%s", secret, res.Body)
		}
	}
	// The expiry digits that follow the PAN in track 2 must not survive either.
	if strings.Contains(res.Body, "D29122011234567890") {
		t.Fatalf("view describe leaked track 2 expiry/discretionary:\n%s", res.Body)
	}
}

// packPAN packs a starter-spec authorization request whose field 2 carries pan
// verbatim, control bytes included.
func packPAN(t *testing.T, spec *iso8583.MessageSpec, pan string) []byte {
	t.Helper()
	msg := iso8583.NewMessage(spec)
	msg.MTI("0100")
	if err := msg.Field(2, pan); err != nil {
		t.Fatalf("set F2 %q: %v", pan, err)
	}
	if err := msg.Field(11, "123456"); err != nil {
		t.Fatalf("set F11: %v", err)
	}
	raw, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack F2 %q: %v", pan, err)
	}
	return raw
}

// assertTerminalSafe fails when a text view carries a byte a terminal would act
// on: any C0 control other than the line separator, DEL, a C1 control, or a
// byte that is not valid UTF-8 (a raw 0x9B is CSI on an 8-bit terminal).
func assertTerminalSafe(t *testing.T, label, body string) {
	t.Helper()
	if !utf8.ValidString(body) {
		t.Fatalf("%s: output is not valid UTF-8: %q", label, body)
	}
	for i, r := range body {
		if r == '\n' {
			continue
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			t.Fatalf("%s: raw control U+%04X at byte %d reaches the terminal: %q", label, r, i, body)
		}
	}
}

// TestTextViewsMaskThenEscapeControlBytesInPAN pins how every view renders a
// PAN that carries control bytes. The text views (describe, filtered describe)
// mask first and then escape the result in caret notation, on the field's one
// line; the JSON view masks and leaves the escaping to JSON. A newline, tab,
// vertical tab, or form feed must not split the value across describe lines or
// tabwriter cells, since the unmasked tail of a split value was printed as a
// line of its own.
func TestTextViewsMaskThenEscapeControlBytesInPAN(t *testing.T) {
	t.Parallel()
	spec := basei.StarterMessageSpec()
	catalog := basei.DefaultExtensionCatalog()
	pal := render.NewPalette(false)

	cases := []struct {
		name string
		pan  string
	}{
		{"NUL inside the BIN", "00000\x000000000000"},
		{"ESC sequence inside the BIN", "4\x1b[2J11111111111"},
		{"DEL in the last four", "411111111111111\x7f"},
		{"BEL in the masked middle", "4111111\x0711111111"},
		{"newline after the BIN", "411111\n111111111111"},
		{"carriage return after the BIN", "411111\r111111111111"},
		{"tab after the BIN", "411111\t111111111111"},
		{"vertical tab after the BIN", "411111\v111111111111"},
		{"form feed after the BIN", "411111\f111111111111"},
		{"only control bytes", "\x1b\x00\n\t\x7f\x1b\x00\n\t\x7f\x01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw := packPAN(t, spec, tc.pan)
			masked := maskPAN(tc.pan)
			want := render.SanitizeControl(masked)

			for _, filters := range [][]string{nil, {"2"}} {
				res, err := ViewMessage(raw, spec, catalog, "describe", filters, pal, false)
				if err != nil {
					t.Fatalf("describe %v: %v", filters, err)
				}
				label := fmt.Sprintf("describe filters=%v", filters)
				assertTerminalSafe(t, label, res.Body)
				got, ok := describeFieldValue(res.Body, "2")
				if !ok {
					t.Fatalf("%s: no F2 line:\n%s", label, res.Body)
				}
				if got != want {
					t.Fatalf("%s: F2 shown %q, want mask-then-escape %q", label, got, want)
				}
				if strings.Contains(res.Body, "11111111") {
					t.Fatalf("%s: a run of masked PAN digits reached the output:\n%s", label, res.Body)
				}
			}

			res, err := ViewMessage(raw, spec, catalog, "json", nil, pal, false)
			if err != nil {
				t.Fatalf("json: %v", err)
			}
			var payload struct {
				Fields map[string]string `json:"fields"`
			}
			if err := json.Unmarshal([]byte(res.Body), &payload); err != nil {
				t.Fatalf("json: %v\n%s", err, res.Body)
			}
			if payload.Fields["2"] != masked {
				t.Fatalf("json: F2 = %q, want the masked raw value %q", payload.Fields["2"], masked)
			}

			unsafeRes, err := ViewMessage(raw, spec, catalog, "describe", nil, pal, true)
			if err != nil {
				t.Fatalf("describe --unsafe: %v", err)
			}
			assertTerminalSafe(t, "describe --unsafe", unsafeRes.Body)
			if got, _ := describeFieldValue(unsafeRes.Body, "2"); got != render.SanitizeControl(tc.pan) {
				t.Fatalf("describe --unsafe: F2 shown %q, want escaped raw %q", got, render.SanitizeControl(tc.pan))
			}
		})
	}
}

// TestTextViewsEscapeNonPrintableFreeText pins the same terminal safety for a
// non-sensitive free-text field (41) in every text view, including the
// filtered one; for bytes that are not ASCII controls (a C1 control such as
// CSI U+009B, and invalid UTF-8 from a field whose spec reads raw bytes); and
// for the MTI, which moov prints unfiltered. The filtered view renders from the
// document, where a binary-encoded field is hex, so it shows wantFiltered.
func TestTextViewsEscapeNonPrintableFreeText(t *testing.T) {
	t.Parallel()
	starter := basei.StarterMessageSpec()
	binary41 := &iso8583.MessageSpec{Name: "binary 41", Fields: maps.Clone(starter.Fields)}
	binary41.Fields[41] = field.NewString(&field.Spec{
		Length:      8,
		Description: "Card Acceptor Terminal Identification",
		Enc:         encoding.Binary,
		Pref:        prefix.Binary.Fixed,
	})
	pal := render.NewPalette(false)

	cases := []struct {
		name         string
		spec         *iso8583.MessageSpec
		mti          string
		value        string
		want         string
		wantFiltered string
	}{
		{"ESC color sequence", starter, "0100", "TE\x1b[31mX", "TE^[[31mX", "TE^[[31mX"},
		{"newline and tab", starter, "0100", "T\nE\tRM\r1", "T^JE^IRM^M1", "T^JE^IRM^M1"},
		{"C1 CSI", binary41, "0100", "TE\u009b2JXY", "TEM-^[2JXY", "5445C29B324A5859"},
		{"invalid UTF-8", binary41, "0100", "TERM\xff\x9b12", "TERMM-^?M-^[12", "5445524DFF9B3132"},
		{"ESC sequence in the MTI", starter, "\x1b[2J", "TERM0001", "TERM0001", "TERM0001"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := iso8583.NewMessage(tc.spec)
			msg.MTI(tc.mti)
			if err := msg.Field(41, tc.value); err != nil {
				t.Fatalf("set F41: %v", err)
			}
			raw, err := msg.Pack()
			if err != nil {
				t.Fatalf("pack: %v", err)
			}
			for _, filters := range [][]string{nil, {"41"}} {
				for _, unsafe := range []bool{false, true} {
					res, err := ViewMessage(raw, tc.spec, basei.DefaultExtensionCatalog(), "describe", filters, pal, unsafe)
					if err != nil {
						t.Fatalf("describe: %v", err)
					}
					label := fmt.Sprintf("describe filters=%v unsafe=%v", filters, unsafe)
					assertTerminalSafe(t, label, res.Body)
					assertTerminalSafe(t, label+" summary", res.Summary)
					want := tc.want
					if filters != nil {
						want = tc.wantFiltered
					}
					if got, _ := describeFieldValue(res.Body, "41"); got != want {
						t.Fatalf("%s: F41 shown %q, want %q", label, got, want)
					}
				}
			}
		})
	}
}

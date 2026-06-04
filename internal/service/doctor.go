package service

import (
	"fmt"
	"strconv"

	"github.com/moov-io/iso8583"

	"github.com/nao1215/iso8583tool/internal/basei"
)

// SpecCandidate reports how well one built-in preset fits a message.
type SpecCandidate struct {
	Preset string `json:"preset"`
	Title  string `json:"title"`
	// Unpacks reports whether the message unpacked without error.
	Unpacks bool `json:"unpacks"`
	// ExactRoundTrip reports whether re-packing the unpacked message reproduces
	// the input byte length. It is the strongest fit signal: a wrong spec that
	// happens to unpack usually re-packs to a different length.
	ExactRoundTrip bool `json:"exact_round_trip"`
	// MTI is the decoded MTI when the message unpacked.
	MTI string `json:"mti,omitempty"`
	// FieldCount is the number of fields decoded (excluding the MTI).
	FieldCount int `json:"field_count"`
	// Detail is a one-line, human-readable note: a fit summary on success or the
	// field-aware failure cause when the message did not unpack.
	Detail string `json:"detail"`
	// Score ranks candidates; higher is a better fit. Reported for transparency.
	Score int `json:"score"`
}

// SpecDiagnosis is the result of trying every built-in preset against a message.
type SpecDiagnosis struct {
	Bytes int `json:"bytes"`
	// InputEncoding records how the input bytes were read (hex or raw). It is
	// filled by the caller and is most useful when the encoding was auto-detected.
	InputEncoding string `json:"input_encoding,omitempty"`
	// Recommended is the --spec value of the best-fitting preset, or empty when
	// no preset could unpack the message.
	Recommended string `json:"recommended,omitempty"`
	// Ambiguous reports that more than one preset fits about equally well, so the
	// recommendation should be confirmed by eye.
	Ambiguous  bool            `json:"ambiguous"`
	Candidates []SpecCandidate `json:"candidates"`
}

// BestScore returns the highest candidate score, or 0 when no preset fit. It
// lets a caller compare two readings of the same bytes (for example hex vs raw)
// and keep the stronger fit when a cheap byte-level heuristic cannot tell them
// apart.
func (d SpecDiagnosis) BestScore() int {
	best := 0
	for _, c := range d.Candidates {
		if c.Score > best {
			best = c.Score
		}
	}
	return best
}

// DiagnoseSpec tries each built-in preset against raw message bytes and ranks
// them by fit. It is a detection aid for picking --spec, not a guarantee:
// distinct specs can unpack the same bytes, so the result names what to confirm
// with `view`. Custom JSON specs are out of scope and not considered here.
func DiagnoseSpec(raw []byte) SpecDiagnosis {
	diag := SpecDiagnosis{Bytes: len(raw)}

	best := -1
	bestScore := 0
	tieScore := 0
	for _, preset := range basei.Presets() {
		c := scorePreset(preset, raw)
		diag.Candidates = append(diag.Candidates, c)
		switch {
		case c.Score > bestScore:
			tieScore = bestScore
			bestScore = c.Score
			best = len(diag.Candidates) - 1
		case c.Score > tieScore:
			tieScore = c.Score
		}
	}

	if best >= 0 && bestScore > 0 {
		diag.Recommended = diag.Candidates[best].Preset
		// Two presets that both unpack with the same strength (for example a
		// plain-ASCII message fits both basei-starter and spec87ascii) are an
		// ambiguous pair worth flagging.
		diag.Ambiguous = tieScore == bestScore
	}
	return diag
}

// scorePreset attempts to unpack raw with one preset and turns the outcome into
// a scored candidate. The scoring favors, in order: an exact byte-length round
// trip, a clean unpack, a valid MTI, and more decoded fields.
func scorePreset(preset basei.Preset, raw []byte) SpecCandidate {
	c := SpecCandidate{Preset: preset.Name, Title: preset.Title}

	msg := iso8583.NewMessage(preset.Spec())
	if err := safeUnpack(msg, raw); err != nil {
		c.Detail = diagnoseUnpack(err, raw).String()
		return c
	}
	c.Unpacks = true
	c.Score = 100

	if mti, err := msg.GetMTI(); err == nil {
		c.MTI = mti
		if validMTI(mti) {
			c.Score += 30
		}
	}

	// GetFields includes the MTI (field 0) and the bitmap; report the count of
	// data fields so the number matches what a user sees.
	fields := msg.GetFields()
	for id := range fields {
		if id != 0 && id != 1 {
			c.FieldCount++
		}
	}
	c.Score += c.FieldCount * 2

	if packed, err := safePack(msg); err == nil && len(packed) == len(raw) {
		c.ExactRoundTrip = true
		c.Score += 200
	}

	c.Detail = fitDetail(c)
	return c
}

func fitDetail(c SpecCandidate) string {
	detail := "MTI " + c.MTI
	if c.MTI == "" {
		detail = "unpacked, no MTI"
	}
	detail += ", " + plural(c.FieldCount, "field", "fields")
	if c.ExactRoundTrip {
		detail += ", exact byte-length round trip"
	}
	return detail
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

func validMTI(mti string) bool {
	if len(mti) != 4 {
		return false
	}
	for _, r := range mti {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// safePack mirrors safeUnpack: it turns a panic from the underlying library
// (some unpacked-but-mismatched messages cannot be re-packed) into an error so
// the doctor never crashes while probing.
func safePack(msg *iso8583.Message) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("%v", r)
		}
	}()
	return msg.Pack()
}

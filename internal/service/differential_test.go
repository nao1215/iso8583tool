package service

import (
	"bytes"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	reference "github.com/mofax/iso8583"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
)

// The differential tests compare the ISO 8583:1987 ASCII preset with
// github.com/mofax/iso8583, an independent implementation that ships its own
// field table (spec1987.yml) and its own packer and parser. Neither side is
// treated as correct: a difference is judged against ISO 8583:1987 and either
// fixed or listed in knownFieldTableDifferences with the reason.

// referenceField is one entry of the reference implementation's field table.
type referenceField struct {
	ContentType string `yaml:"ContentType"`
	LenType     string `yaml:"LenType"`
	MaxLen      int    `yaml:"MaxLen"`
	MinLen      int    `yaml:"MinLen"`
	Label       string `yaml:"Label"`
}

// knownFieldTableDifferences lists the fields whose definition differs from the
// reference on purpose. The table test fails when an entry stops differing, so
// the list cannot go stale.
var knownFieldTableDifferences = map[int]string{
	// ISO 8583:1987 defines field 44 (additional response data) as an..25. The
	// preset inherits moov-io's Spec87ASCII, which accepts up to 99 characters,
	// so a longer value from a non-conforming host still unpacks.
	44: "llvar up to 99 instead of 25",
}

var (
	referenceSpecOnce   sync.Once
	referenceSpecPath   string
	referenceSpecFields map[int]referenceField
	errReferenceSpec    error
)

// loadReferenceSpec returns the path of the reference spec1987.yml in the module
// cache and its parsed field table.
func loadReferenceSpec(t *testing.T) (string, map[int]referenceField) {
	t.Helper()
	referenceSpecOnce.Do(func() {
		out, err := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{.Dir}}", "github.com/mofax/iso8583").Output()
		if err != nil {
			errReferenceSpec = err
			return
		}
		dir := strings.TrimSpace(string(out))
		referenceSpecPath = filepath.Join(dir, "spec1987.yml")
		data, err := fs.ReadFile(os.DirFS(dir), "spec1987.yml")
		if err != nil {
			errReferenceSpec = err
			return
		}
		errReferenceSpec = yaml.Unmarshal(data, &referenceSpecFields)
	})
	if errReferenceSpec != nil {
		t.Fatalf("load the reference field table: %v", errReferenceSpec)
	}
	return referenceSpecPath, referenceSpecFields
}

// presetFieldShape describes a field of the preset in the reference's terms:
// "fixed", "llvar" or "lllvar", the maximum length, and whether the field holds
// bytes rather than characters.
func presetFieldShape(t *testing.T, id int) (lenType string, length int, binary bool) {
	t.Helper()
	f, ok := basei.Spec87ASCIIWithSecondaryFields().Fields[id]
	if !ok {
		t.Fatalf("field %d is not defined by the ISO 8583:1987 ASCII preset", id)
	}
	s := f.Spec()
	switch pref := s.Pref.Inspect(); pref {
	case "ASCII.Fixed":
		return "fixed", s.Length, false
	case "Hex.Fixed", "Binary.Fixed":
		return "fixed", s.Length, true
	case "ASCII.LL":
		return "llvar", s.Length, false
	case "ASCII.LLL":
		return "lllvar", s.Length, false
	default:
		t.Fatalf("field %d: unexpected prefix %s", id, pref)
		return "", 0, false
	}
}

func TestDifferentialFieldTableMatchesTheReference(t *testing.T) {
	t.Parallel()
	_, ref := loadReferenceSpec(t)

	// Field 1 is the secondary bitmap and field 65 the tertiary bitmap
	// indicator; both are handled by the bitmap, not as data fields.
	for id := 2; id <= 128; id++ {
		if id == 65 {
			continue
		}
		t.Run("field "+strconv.Itoa(id), func(t *testing.T) {
			t.Parallel()
			want, ok := ref[id]
			if !ok {
				t.Fatalf("the reference does not define field %d", id)
			}
			lenType, length, binary := presetFieldShape(t, id)
			same := lenType == want.LenType && length == want.MaxLen && binary == (want.ContentType == "b")
			reason, known := knownFieldTableDifferences[id]
			switch {
			case !same && !known:
				t.Errorf("preset defines field %d as %s %d (binary=%t), the reference as %s %d %s (%s)",
					id, lenType, length, binary, want.LenType, want.MaxLen, want.ContentType, want.Label)
			case same && known:
				t.Errorf("field %d now matches the reference; remove it from knownFieldTableDifferences (%s)", id, reason)
			}
		})
	}
}

// differentialFieldIDs returns the text fields whose definition agrees with the
// reference, the ones a generated message may carry.
func differentialFieldIDs(t *testing.T) []int {
	t.Helper()
	_, ref := loadReferenceSpec(t)
	var ids []int
	for id := 2; id <= 128; id++ {
		if id == 65 {
			continue
		}
		if _, known := knownFieldTableDifferences[id]; known {
			continue
		}
		lenType, length, binary := presetFieldShape(t, id)
		want := ref[id]
		if binary || lenType != want.LenType || length != want.MaxLen {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// drawFieldValue draws a value that the reference's content type allows and
// whose length the field accepts. Variable fields stay short so a message with
// many fields remains readable in a failure message.
func drawFieldValue(t *rapid.T, id int, f referenceField) string {
	label := "field" + strconv.Itoa(id)
	length := f.MaxLen
	if f.LenType != "fixed" {
		lo := max(f.MinLen, 1)
		length = rapid.IntRange(lo, max(lo, min(f.MaxLen, 40))).Draw(t, label+"len")
	}
	var class string
	switch f.ContentType {
	case "n":
		class = "[0-9]"
	case "z":
		class = "[0-9=]"
	case "an":
		class = "[0-9A-Za-z]"
	default: // ans, ns
		class = "[ -~]"
	}
	return rapid.StringMatching(class+"{"+strconv.Itoa(length)+"}").Draw(t, label)
}

// referencePack packs the message with the reference implementation.
func referencePack(t *rapid.T, specPath, mti string, fields map[int]string) string {
	secondary := slices.ContainsFunc(slices.Collect(maps.Keys(fields)), func(id int) bool { return id > 64 })
	msg := reference.NewISOStruct(specPath, secondary)
	if err := msg.AddMTI(mti); err != nil {
		t.Fatalf("reference AddMTI(%q): %v", mti, err)
	}
	for id, v := range fields {
		if err := msg.AddField(int64(id), v); err != nil {
			t.Fatalf("reference AddField(%d): %v", id, err)
		}
	}
	wire, err := msg.ToString()
	if err != nil {
		t.Fatalf("reference ToString: %v", err)
	}
	return wire
}

// drawDifferentialMessage draws an MTI and a set of fields both sides accept.
func drawDifferentialMessage(t *rapid.T, ids []int, ref map[int]referenceField) (string, map[int]string) {
	mti := rapid.StringMatching(`[0-9]{4}`).Draw(t, "mti")
	chosen := rapid.SliceOfNDistinct(rapid.SampledFrom(ids), 1, 12, func(id int) int { return id }).Draw(t, "ids")
	fields := make(map[int]string, len(chosen))
	for _, id := range chosen {
		fields[id] = drawFieldValue(t, id, ref[id])
	}
	return mti, fields
}

func documentOf(mti string, fields map[int]string) messageio.Document {
	doc := messageio.Document{MTI: mti, Fields: map[string]string{}}
	for id, v := range fields {
		doc.Fields[strconv.Itoa(id)] = v
	}
	return doc
}

// The preset and the reference write the bitmap differently, and only the
// bitmap. ISO 8583:1987 sends the 64-bit primary bitmap and adds the secondary
// one only when bit 1 is set. The preset inherits moov-io's Spec87ASCII bitmap,
// a single 128-bit block that is always sent and never sets bit 1, so a
// message with only primary fields carries 16 extra zero hex digits and a
// message with a secondary field leaves bit 1 clear. The reference also writes
// lower-case hex digits. The two helpers below translate between the forms so
// that every other byte is compared exactly.
const (
	mtiLen       = 4
	primaryHex   = 16
	secondaryHex = 32
)

// toPresetBitmap rewrites an ISO 8583 message into the preset's bitmap form.
func toPresetBitmap(wire string) []byte {
	bitmap, rest := wire[mtiLen:mtiLen+primaryHex], wire[mtiLen+primaryHex:]
	first, err := strconv.ParseUint(bitmap[:1], 16, 8)
	if err != nil {
		panic(err)
	}
	if first&0x8 == 0 {
		bitmap += strings.Repeat("0", primaryHex)
	} else {
		bitmap = strconv.FormatUint(first&^0x8, 16) + bitmap[1:] + rest[:primaryHex]
		rest = rest[primaryHex:]
	}
	return []byte(wire[:mtiLen] + strings.ToUpper(bitmap) + rest)
}

// toISOBitmap rewrites a message the preset packed into the ISO 8583 form.
func toISOBitmap(wire []byte) string {
	bitmap, rest := string(wire[mtiLen:mtiLen+secondaryHex]), string(wire[mtiLen+secondaryHex:])
	if bitmap[primaryHex:] == strings.Repeat("0", primaryHex) {
		bitmap = bitmap[:primaryHex]
	} else {
		first, err := strconv.ParseUint(bitmap[:1], 16, 8)
		if err != nil {
			panic(err)
		}
		bitmap = strconv.FormatUint(first|0x8, 16) + bitmap[1:]
	}
	return string(wire[:mtiLen]) + bitmap + rest
}

func TestDifferentialPackMatchesTheReference(t *testing.T) {
	t.Parallel()
	specPath, ref := loadReferenceSpec(t)
	ids := differentialFieldIDs(t)
	spec := basei.Spec87ASCIIWithSecondaryFields()

	rapid.Check(t, func(t *rapid.T) {
		mti, fields := drawDifferentialMessage(t, ids, ref)
		ours, err := WriteMessage(documentOf(mti, fields), spec)
		if err != nil {
			t.Fatalf("WriteMessage rejected a message the reference packs: %v\nfields=%v", err, fields)
		}
		theirs := referencePack(t, specPath, mti, fields)
		if want := toPresetBitmap(theirs); !bytes.Equal(ours.Raw, want) {
			t.Fatalf("packed messages differ\n ours=%q\ntheirs=%q (%q in the preset's bitmap form)\nfields=%v", ours.Raw, theirs, want, fields)
		}
	})
}

func TestDifferentialUnpackAgreesWithTheReference(t *testing.T) {
	t.Parallel()
	specPath, ref := loadReferenceSpec(t)
	ids := differentialFieldIDs(t)
	spec := basei.Spec87ASCIIWithSecondaryFields()

	rapid.Check(t, func(t *rapid.T) {
		mti, fields := drawDifferentialMessage(t, ids, ref)
		want := documentOf(mti, fields)

		// The preset reads what the reference wrote.
		theirs := referencePack(t, specPath, mti, fields)
		got, err := MessageToDocument(spec, toPresetBitmap(theirs))
		if err != nil {
			t.Fatalf("MessageToDocument rejected the reference's message %q: %v", theirs, err)
		}
		if got.MTI != want.MTI || !maps.Equal(got.Fields, want.Fields) || len(got.BinaryFields) != 0 {
			t.Fatalf("unpacked the reference's message as\n got=%#v\nwant=%#v", got, want)
		}

		// The reference reads what the preset wrote.
		ours, err := WriteMessage(want, spec)
		if err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
		parser := reference.NewISOStruct(specPath, false)
		parsed, err := parser.Parse(toISOBitmap(ours.Raw))
		if err != nil {
			t.Fatalf("the reference rejected the preset's message %q: %v", ours.Raw, err)
		}
		if parsed.Mti.String() != mti {
			t.Fatalf("the reference read MTI %q, want %q", parsed.Mti.String(), mti)
		}
		elements := parsed.Elements.GetElements()
		if len(elements) != len(fields) {
			t.Fatalf("the reference read %d fields, want %d: %v", len(elements), len(fields), elements)
		}
		for id, v := range fields {
			if elements[int64(id)] != v {
				t.Fatalf("the reference read field %d as %q, want %q", id, elements[int64(id)], v)
			}
		}
	})
}

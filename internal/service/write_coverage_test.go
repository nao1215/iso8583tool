package service

import (
	"strings"
	"testing"

	"github.com/nao1215/iso8583tool/internal/basei"
	"github.com/nao1215/iso8583tool/internal/messageio"
)

// TestWriteMessageErrorBranches exercises the reachable input-validation error
// paths of WriteMessage, which a valid document never hits.
func TestWriteMessageErrorBranches(t *testing.T) {
	t.Parallel()

	spec := basei.StarterMessageSpec()

	cases := []struct {
		name string
		doc  messageio.Document
		want string
	}{
		{
			name: "non-numeric field id",
			doc:  messageio.Document{MTI: "0100", Fields: map[string]string{"abc": "1"}},
			want: "invalid field id",
		},
		{
			name: "bad dot-path on plain field",
			doc:  messageio.Document{MTI: "0100", Fields: map[string]string{"3.5": "1"}},
			want: "3.5",
		},
		{
			name: "text field via binary_fields",
			doc:  messageio.Document{MTI: "0100", BinaryFields: map[string]string{"3": "303030303030"}},
			want: "text field",
		},
		{
			name: "non-numeric binary field id",
			doc:  messageio.Document{MTI: "0100", BinaryFields: map[string]string{"zz": "AABB"}},
			want: "invalid field id",
		},
		{
			name: "bad hex in binary field",
			doc:  messageio.Document{MTI: "0100", BinaryFields: map[string]string{"52": "nothex"}},
			want: "decode binary field",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := WriteMessage(c.doc, spec)
			if err == nil {
				t.Fatalf("%s: expected an error", c.name)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s: err = %q, want to contain %q", c.name, err.Error(), c.want)
			}
		})
	}
}

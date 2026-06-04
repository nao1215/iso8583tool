package service

import (
	"sort"
	"strconv"
	"strings"

	"github.com/moov-io/iso8583"

	"github.com/nao1215/iso8583tool/internal/messageio"
)

// DiffKind classifies a field-level change between two messages.
type DiffKind string

const (
	DiffAdded   DiffKind = "added"
	DiffRemoved DiffKind = "removed"
	DiffChanged DiffKind = "changed"
)

// DiffEntry is a single field difference, keyed by its logical path.
type DiffEntry struct {
	Path   string   `json:"path"`
	Kind   DiffKind `json:"kind"`
	Before string   `json:"before,omitempty"`
	After  string   `json:"after,omitempty"`
}

// DiffResult is the ordered set of differences between two messages.
type DiffResult struct {
	Changes []DiffEntry `json:"changes"`
}

// DiffMessages unpacks two messages and compares them by logical field path
// (including nested EMV tags such as 55.9F02). Values are the canonical,
// padded representations, so the comparison is stable. filters, when set, keep
// only paths equal to or nested under one of the given paths.
//
// Differences are detected on the real field values, but unless unsafe is set
// the displayed values are masked exactly as view masks them, so diff output is
// safe to paste into a ticket or pipe to jq. unsafe restores the raw values for
// local debugging.
func DiffMessages(spec *iso8583.MessageSpec, before, after []byte, filters []string, unsafe bool) (DiffResult, error) {
	beforeDoc, err := MessageToDocument(spec, before)
	if err != nil {
		return DiffResult{}, err
	}
	afterDoc, err := MessageToDocument(spec, after)
	if err != nil {
		return DiffResult{}, err
	}

	beforeMap := FlattenDocument(beforeDoc)
	afterMap := FlattenDocument(afterDoc)

	mask := func(_, value string) string { return value }
	if !unsafe {
		unknownPaths := diffUnknownPaths(spec, before, after)
		mask = func(path, value string) string { return maskValueForDiff(path, value, unknownPaths) }
	}

	paths := unionPaths(beforeMap, afterMap)
	sortPaths(paths)

	result := DiffResult{}
	for _, path := range paths {
		if !matchesAnyFilter(path, filters) {
			continue
		}
		b, okB := beforeMap[path]
		a, okA := afterMap[path]
		switch {
		case okB && okA:
			if b != a {
				result.Changes = append(result.Changes, DiffEntry{Path: path, Kind: DiffChanged, Before: mask(path, b), After: mask(path, a)})
			}
		case okB:
			result.Changes = append(result.Changes, DiffEntry{Path: path, Kind: DiffRemoved, Before: mask(path, b)})
		case okA:
			result.Changes = append(result.Changes, DiffEntry{Path: path, Kind: DiffAdded, After: mask(path, a)})
		}
	}
	return result, nil
}

// diffUnknownPaths returns the set of Field 55 tag paths that neither message
// maps to a known spec field, so diff can mask their bytes like view does.
func diffUnknownPaths(spec *iso8583.MessageSpec, before, after []byte) map[string]bool {
	unknown := map[string]bool{}
	for _, raw := range [][]byte{before, after} {
		msg := iso8583.NewMessage(spec)
		if err := safeUnpack(msg, raw); err != nil {
			continue
		}
		for _, t := range collectUnknownTags(msg) {
			unknown[t.Path] = true
		}
	}
	return unknown
}

// FlattenDocument collapses a message document into a single path->value map.
// The MTI is keyed as "mti"; text and binary fields keep their dot-paths.
func FlattenDocument(doc messageio.Document) map[string]string {
	flat := make(map[string]string, len(doc.Fields)+len(doc.BinaryFields)+1)
	if doc.MTI != "" {
		flat["mti"] = doc.MTI
	}
	for k, v := range doc.Fields {
		flat[k] = v
	}
	for k, v := range doc.BinaryFields {
		flat[k] = v
	}
	return flat
}

func unionPaths(a, b map[string]string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	paths := make([]string, 0, len(a)+len(b))
	for k := range a {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			paths = append(paths, k)
		}
	}
	for k := range b {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			paths = append(paths, k)
		}
	}
	return paths
}

// matchesAnyFilter reports whether path is selected by the filters. An empty
// filter list matches everything; otherwise a path matches a filter when it is
// equal to it or nested beneath it (filter "55" matches "55.9F02").
func matchesAnyFilter(path string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if path == f || strings.HasPrefix(path, f+".") {
			return true
		}
	}
	return false
}

// sortPaths orders paths deterministically: "mti" first, then by numeric field
// id, then lexically within each nested level.
func sortPaths(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		return comparePaths(paths[i], paths[j]) < 0
	})
}

func comparePaths(a, b string) int {
	if a == b {
		return 0
	}
	if a == "mti" {
		return -1
	}
	if b == "mti" {
		return 1
	}
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		ai, aerr := strconv.Atoi(as[i])
		bi, berr := strconv.Atoi(bs[i])
		if aerr == nil && berr == nil {
			if ai != bi {
				return ai - bi
			}
			continue
		}
		return strings.Compare(as[i], bs[i])
	}
	return len(as) - len(bs)
}

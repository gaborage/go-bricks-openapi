package analyzer

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"reflect"
	"strings"
)

// scannedTagKeys are the struct-tag keys this tool reads. A malformed tag that
// hides one of these costs the emitted spec real metadata; a malformed tag that
// hides none of them costs us nothing, and `go vet` is the right tool for that.
// Keep in sync with parseStructTags.
var scannedTagKeys = []string{
	tagJSON, tagParam, tagQuery, tagHeader, tagDoc, tagExample, tagValidate,
}

// isTagKeyByte reports whether b may appear in a struct-tag KEY. This mirrors
// reflect.StructTag's own scan exactly — it is far wider than [A-Za-z0-9_-.],
// which is why an earlier, narrower guess false-positived on keys like
// `x/json:` and `$header:`.
func isTagKeyByte(b byte) bool {
	return b > ' ' && b != ':' && b != '"' && b != 0x7f
}

// tagScanEnd returns the offset at which reflect.StructTag's scan stops. A return
// of len(tag) means reflect consumed the whole tag, so there is no unread
// remainder for hiddenTagKeys to search — which is why a value that merely ends
// in `<key>:` (e.g. `doc:"see example:"`) is not reported.
//
// This mirrors reflect's SCAN but not its value decoding: reflect additionally
// breaks on a strconv.Unquote error for the key it is looking up, so a tag like
// `json:"\x" validate:"required"` scans to the end here while Lookup("json")
// still fails. That is a false NEGATIVE (we stay silent), never a false
// positive, and `go vet` does flag it.
func tagScanEnd(tag string) int {
	i := 0
	for i < len(tag) {
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		if i == len(tag) {
			return i
		}
		start := i
		for i < len(tag) && isTagKeyByte(tag[i]) {
			i++
		}
		if i == start || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			return start
		}
		end := tagValueEnd(tag, i+1)
		if end < 0 {
			return start
		}
		i = end
	}
	return i
}

// tagValueEnd returns the offset just past the closing quote of the value whose
// opening quote is at q, or -1 if the value is unterminated. Extracted from
// tagScanEnd purely to keep that function under the complexity ceilings —
// inlined it measures gocognit 20, over SonarCloud's S3776 limit of 15.
func tagValueEnd(tag string, q int) int {
	i := q + 1
	for i < len(tag) && tag[i] != '"' {
		if tag[i] == '\\' {
			i++
		}
		i++
	}
	if i >= len(tag) {
		return -1
	}
	return i + 1
}

// hiddenTagKeys returns the scanned keys that tagText appears to set but reflect
// cannot read, in a stable order.
func hiddenTagKeys(tagText string) []string {
	end := tagScanEnd(tagText)
	if end >= len(tagText) {
		return nil
	}
	rest := tagText[end:]
	var hidden []string
	for _, key := range scannedTagKeys {
		if _, ok := reflect.StructTag(tagText).Lookup(key); ok {
			continue
		}
		if keyStartsIn(rest, key) {
			hidden = append(hidden, key)
		}
	}
	return hidden
}

// tagTokenAt skips non-key bytes from i, then returns the bounds of the run of
// key bytes that follows. start == end means no token remains.
func tagTokenAt(s string, i int) (start, end int) {
	for i < len(s) && !isTagKeyByte(s[i]) {
		i++
	}
	start = i
	for i < len(s) && isTagKeyByte(s[i]) {
		i++
	}
	return start, i
}

// keyStartsIn reports whether key appears as a KEY in rest. It walks rest as a
// sequence of key:"value" pairs and skips every parsed value region, so a key
// name that merely appears inside a hidden value is not matched — `doc:"see
// query:"` hides doc, not query.
func keyStartsIn(rest, key string) bool {
	for i := 0; i < len(rest); {
		start, end := tagTokenAt(rest, i)
		if start == end {
			return false
		}
		i = end
		if i+1 >= len(rest) || rest[i] != ':' || rest[i+1] != '"' {
			continue
		}
		if rest[start:i] == key {
			return true
		}
		v := tagValueEnd(rest, i+1)
		if v < 0 {
			return false
		}
		i = v
	}
	return false
}

// relToRoot renders a path relative to the analyzed project root for display,
// falling back to the absolute path when that is not possible.
func relToRoot(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

// warnUnreadableTagKeys emits one diagnostic per malformed tag, deduped by source
// position so a tag reached from more than one path warns once.
func (a *ProjectAnalyzer) warnUnreadableTagKeys(tag *ast.BasicLit, fieldName, tagText string) {
	if tag == nil {
		return
	}
	hidden := hiddenTagKeys(tagText)
	if len(hidden) == 0 {
		return
	}
	pos := a.fileSet.Position(tag.Pos())
	loc := fmt.Sprintf("%s:%d:%d", relToRoot(a.projectRoot, pos.Filename), pos.Line, pos.Column)
	if a.tagWarned == nil {
		a.tagWarned = make(map[string]struct{})
	}
	if _, seen := a.tagWarned[loc]; seen {
		return
	}
	a.tagWarned[loc] = struct{}{}
	a.addWarningf("struct tag on field %s at %s is not readable by reflect.StructTag, so %s dropped from the spec; "+
		"key:\"value\" pairs must be separated by single spaces — run `go vet ./...` to see the exact error (go test does not check struct tags)",
		fieldName, loc, strings.Join(hidden, ", "))
}

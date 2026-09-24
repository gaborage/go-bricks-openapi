package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Directive parsing: one parser, one namespace, one placement rule.
//
// A Directive is a `//openapi:<name>[ <args>]` line inside a comment group whose
// LAST line sits immediately above a route registration call. A group that
// starts after code on the same line (`server.GET(...) //openapi:public`) never
// attaches. Anything after a second `//` on the directive line is a human
// comment and is ignored.
//
// Recognised names:
//   - public          — zero arguments; marks the route as needing no auth.
//   - errors 404,409  — a single-line, comma-separated list of extra error
//     statuses the operation can return (400-599).
//
// An unrecognised name, an argument on `public`, and a malformed or
// out-of-range `errors` token each raise an analyzer diagnostic, which feeds
// `--strict`. So does a Detached directive: a group holding a recognised name
// that attached to no registration the walk recognised (see
// warnDetachedDirectives).
const (
	directivePrefix = "//openapi:"

	directiveNamePublic = "public"
	directiveNameErrors = "errors"

	// errorStatusMin/errorStatusMax bound the status codes `errors` accepts —
	// the HTTP client- and server-error ranges.
	errorStatusMin = 400
	errorStatusMax = 599
)

// routeDirectives is the parsed Directive payload attached to one registration
// site (a comment group ending directly above the call).
type routeDirectives struct {
	public bool
	// errorStatuses are the extra error status codes declared via
	// `//openapi:errors`, in source order. Duplicates are possible; the
	// generator deduplicates against the baseline and among themselves.
	errorStatuses []int
}

// directiveGroup is one indexed comment group carrying at least one Directive:
// its parsed payload plus what the detached-directive pass needs to report it.
type directiveGroup struct {
	rd routeDirectives
	// names are the recognised Directive names in the group, deduplicated, in
	// source order. Empty when every name was unknown: such a group is already
	// diagnosed as unknown and is never also reported as detached.
	names []string
	// at is the group's first recognised Directive — where a detached warning
	// points.
	at token.Pos
	// trailing is true when the group shares its first line with preceding code
	// (`server.GET(...) //openapi:public`). Such a group never attaches, not even
	// to a registration on the next line.
	trailing bool
	// attached is set once a registration the walk recognised sits directly
	// below the group. Idempotent: a registration reached twice attaches once.
	attached bool
}

// parseDirective splits one comment line into a Directive name and its raw
// argument string. ok is false when the line is not an `//openapi:` directive.
// A trailing `// ...` human comment after the arguments is discarded.
//
// The name ends at the first whitespace RUNE, not at an ASCII space: a tab
// between the name and its arguments is as natural as a space in Go source, and
// splitting on " " alone would read `//openapi:errors<TAB>404` as a directive
// literally named "errors\t404" — reported as unknown, with the declaration
// silently dropped.
func parseDirective(text string) (name, args string, ok bool) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, directivePrefix) {
		return "", "", false
	}
	rest := t[len(directivePrefix):]
	if i := strings.Index(rest, "//"); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.TrimSpace(rest)
	if i := strings.IndexFunc(rest, unicode.IsSpace); i >= 0 {
		return rest[:i], strings.TrimSpace(rest[i:]), true
	}
	return rest, "", true
}

// indexDirectives records, for every comment group carrying at least one
// Directive, the parsed group keyed by the file line the group ends on.
// Registration recognition consumes that index when a group ends directly
// above a registration call, and warnDetachedDirectives reports what it never
// consumed. Position-keyed (file+line) so it works regardless of which file a
// walked body lives in (helpers and delegates may be cross-file), and
// idempotent across re-parses of the same file: a file is indexed once, so its
// diagnostics are never emitted twice.
func (a *ProjectAnalyzer) indexDirectives(astFile *ast.File) {
	if a.directiveFiles == nil {
		a.directiveFiles = map[string]struct{}{}
	}
	if a.directives == nil {
		a.directives = map[string]map[int]*directiveGroup{}
	}
	filename := a.fileSet.Position(astFile.Pos()).Filename
	if _, seen := a.directiveFiles[filename]; seen {
		return
	}
	a.directiveFiles[filename] = struct{}{}

	var codeStarts map[int]token.Pos // built on the first group that needs it
	for _, cg := range astFile.Comments {
		g := a.parseDirectiveGroup(cg)
		if g == nil {
			continue
		}
		// Only a group that can attach something needs the trailing check: an
		// unknown-only group carries no payload and is never reported detached.
		if len(g.names) > 0 {
			if codeStarts == nil {
				codeStarts = a.firstCodeByLine(astFile)
			}
			start, ok := codeStarts[a.fileSet.Position(cg.Pos()).Line]
			g.trailing = ok && start <= cg.Pos()
		}
		pos := a.fileSet.Position(cg.End())
		a.directiveLines(pos.Filename)[pos.Line] = g
	}
}

// directiveLines returns filename's end-line -> group index, creating it on
// first use.
func (a *ProjectAnalyzer) directiveLines(filename string) map[int]*directiveGroup {
	lines := a.directives[filename]
	if lines == nil {
		lines = map[int]*directiveGroup{}
		a.directives[filename] = lines
	}
	return lines
}

// firstCodeByLine maps each line of astFile holding code to the earliest
// position on it where a syntax node begins or ends. A comment starting at or
// after that position shares its line with preceding code: the first token on
// a line of valid Go source always begins or ends a node (a line cannot open
// with a bare `,` or operator — semicolon insertion forbids it), so code
// before the comment always leaves a boundary there.
func (a *ProjectAnalyzer) firstCodeByLine(astFile *ast.File) map[int]token.Pos {
	first := map[int]token.Pos{}
	note := func(p token.Pos) {
		line := a.fileSet.Position(p).Line
		if cur, ok := first[line]; !ok || p < cur {
			first[line] = p
		}
	}
	ast.Inspect(astFile, func(n ast.Node) bool {
		switch n.(type) {
		case nil, *ast.CommentGroup, *ast.Comment:
			return false
		}
		note(n.Pos())
		note(n.End())
		return true
	})
	return first
}

// parseDirectiveGroup folds every Directive line in one comment group into a
// single directiveGroup. It returns nil when the group carries none.
func (a *ProjectAnalyzer) parseDirectiveGroup(cg *ast.CommentGroup) *directiveGroup {
	var g *directiveGroup
	for _, c := range cg.List {
		name, args, ok := parseDirective(c.Text)
		if !ok {
			continue
		}
		if g == nil {
			g = &directiveGroup{}
		}
		if a.applyDirective(&g.rd, name, args, c.Pos()) {
			g.noteRecognised(name, c.Pos())
		}
	}
	return g
}

// noteRecognised records a recognised Directive name on the group, keeping the
// first one's position as the group's location.
func (g *directiveGroup) noteRecognised(name string, pos token.Pos) {
	if len(g.names) == 0 {
		g.at = pos
	}
	if !slices.Contains(g.names, name) {
		g.names = append(g.names, name)
	}
}

// applyDirective folds one parsed Directive into rd, diagnosing an unknown name
// or arguments that the name does not accept. recognised is false for an
// unknown name.
func (a *ProjectAnalyzer) applyDirective(rd *routeDirectives, name, args string, pos token.Pos) (recognised bool) {
	switch name {
	case directiveNamePublic:
		if args != "" {
			a.warnDirectivef(pos, "directive public takes no arguments (got %q)", args)
		}
		rd.public = true
	case directiveNameErrors:
		rd.errorStatuses = append(rd.errorStatuses, a.parseErrorStatuses(args, pos)...)
	default:
		a.warnDirectivef(pos, "unknown directive %q — no such openapi directive", name)
		return false
	}
	return true
}

// parseErrorStatuses parses the comma-separated argument list of
// `//openapi:errors`. Whitespace around tokens is tolerated, and an empty token
// (a trailing or doubled comma) is skipped silently — it names nothing to
// report. Every other token that is not an unsigned integer in
// [400, 599] raises one diagnostic naming it; the remaining valid tokens are
// still returned.
func (a *ProjectAnalyzer) parseErrorStatuses(args string, pos token.Pos) []int {
	if args == "" {
		a.warnDirectivef(pos, "directive errors requires at least one status code (e.g. //openapi:errors 404,409)")
		return nil
	}
	var codes []int
	for _, raw := range strings.Split(args, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		code, ok := parseStatusToken(tok)
		if !ok {
			a.warnDirectivef(pos, "directive errors: ignoring %q — expected an integer status code between %d and %d", tok, errorStatusMin, errorStatusMax)
			continue
		}
		codes = append(codes, code)
	}
	return codes
}

// parseStatusToken converts one `errors` token to a status code. The token must
// be ASCII digits only: strconv.Atoi would otherwise accept a sign, letting
// "+404" through as a valid declaration and reading "-404" as merely
// out-of-range rather than as the malformed token it is.
func parseStatusToken(tok string) (code int, ok bool) {
	for _, r := range tok {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	code, err := strconv.Atoi(tok)
	if err != nil || code < errorStatusMin || code > errorStatusMax {
		return 0, false
	}
	return code, true
}

// warnDirectivef records a Directive diagnostic prefixed with the directive's
// source location. Directive diagnostics feed `--strict`, so a malformed
// directive fails a strict run with no artifact emitted.
func (a *ProjectAnalyzer) warnDirectivef(pos token.Pos, format string, args ...any) {
	p := a.fileSet.Position(pos)
	loc := fmt.Sprintf("%s:%d", relToRoot(a.projectRoot, p.Filename), p.Line)
	a.addWarningf("%s: %s", loc, fmt.Sprintf(format, args...))
}

// directiveGroupAt returns the comment group that attaches to a registration
// call whose first token is at pos — the group ending on the line directly
// above it — or nil when there is none. A trailing group never attaches.
func (a *ProjectAnalyzer) directiveGroupAt(pos token.Pos) *directiveGroup {
	p := a.fileSet.Position(pos)
	g := a.directives[p.Filename][p.Line-1]
	if g == nil || g.trailing {
		return nil
	}
	return g
}

// directivesAt returns the Directives attached to the registration call whose
// first token is at pos. The zero value is returned when there are none.
func (a *ProjectAnalyzer) directivesAt(pos token.Pos) routeDirectives {
	if g := a.directiveGroupAt(pos); g != nil {
		return g.rd
	}
	return routeDirectives{}
}

// markDirectivesAttached records that the call at pos was recognised as a route
// registration, so the group directly above it is attached. Called at
// recognition — before path, prefix, or method resolution — so a directive
// above a registration the walk then drops is not also reported as detached:
// the drop's own diagnostic already covers that route.
func (a *ProjectAnalyzer) markDirectivesAttached(pos token.Pos) {
	if g := a.directiveGroupAt(pos); g != nil {
		g.attached = true
	}
}

// warnDetachedDirectives reports, once per comment group, every recognised
// Directive that attached to no registration the walk recognised — separated
// from its call by a blank line, above an enclosing statement, inside a call,
// after code on the same line, or above a registration the analyzer never
// walks. Only files in walked (those module discovery visited) are policed: a
// file parsed solely to resolve an import or type, such as one in a nested Go
// module discovery skips as another service, is not this service's code. Run
// after the walk; sorted by (file, line) so the output is independent of the
// order files were discovered and indexed in.
func (a *ProjectAnalyzer) warnDetachedDirectives(walked map[string]bool) {
	for _, file := range slices.Sorted(maps.Keys(a.directives)) {
		if !walked[file] {
			continue
		}
		groups := a.directives[file]
		for _, line := range slices.Sorted(maps.Keys(groups)) {
			if g := groups[line]; !g.attached && len(g.names) > 0 {
				a.warnDirectivef(g.at, "%s", detachedDirectiveMessage(g.names))
			}
		}
	}
}

// detachedDirectiveMessage words the detached diagnostic for the recognised
// Directive names of one group.
func detachedDirectiveMessage(names []string) string {
	if len(names) == 1 {
		return "directive " + names[0] + " has no effect — it is not directly above an analyzed route registration"
	}
	return "directives " + strings.Join(names, ", ") + " have no effect — they are not directly above an analyzed route registration"
}

// applyRouteDirectives stamps the Directives attached to the registration call
// at pos onto the route: `public` marks it auth-free, `errors` contributes the
// declared extra error statuses the generator unions with its baseline.
func (a *ProjectAnalyzer) applyRouteDirectives(route *models.Route, pos token.Pos) {
	rd := a.directivesAt(pos)
	if rd.public {
		route.Public = true
	}
	// Unioned with whatever the handler-body walk already inferred, sorted and
	// deduplicated so `404,404` (or two errors directives in one group, or a
	// declaration of a code the handler demonstrably returns) declares 404 once,
	// and the stamped order is deterministic.
	route.ErrorStatuses = mergeErrorStatuses(route.ErrorStatuses, rd.errorStatuses)
}

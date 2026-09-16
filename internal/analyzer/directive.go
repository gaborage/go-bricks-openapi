package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Directive parsing: one parser, one namespace, one placement rule.
//
// A Directive is a `//openapi:<name>[ <args>]` line inside a comment group whose
// LAST line sits immediately above a route registration call. Anything after a
// second `//` on the directive line is a human comment and is ignored.
//
// Recognised names:
//   - public          — zero arguments; marks the route as needing no auth.
//   - errors 404,409  — a single-line, comma-separated list of extra error
//     statuses the operation can return (400-599).
//
// An unrecognised name, an argument on `public`, and a malformed or
// out-of-range `errors` token each raise an analyzer diagnostic, which feeds
// `--strict`.
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
// Directive, the parsed Directives keyed by the file line the group ends on.
// routeFromCall consumes that index when a group ends directly above a
// registration call. Position-keyed (file+line) so it works regardless of which
// file a walked body lives in (helpers and delegates may be cross-file), and
// idempotent across re-parses of the same file: a file is indexed once, so its
// diagnostics are never emitted twice.
func (a *ProjectAnalyzer) indexDirectives(astFile *ast.File) {
	if a.directiveFiles == nil {
		a.directiveFiles = map[string]struct{}{}
	}
	if a.directives == nil {
		a.directives = map[string]map[int]routeDirectives{}
	}
	filename := a.fileSet.Position(astFile.Pos()).Filename
	if _, seen := a.directiveFiles[filename]; seen {
		return
	}
	a.directiveFiles[filename] = struct{}{}

	for _, cg := range astFile.Comments {
		rd, found := a.parseDirectiveGroup(cg)
		if !found {
			continue
		}
		pos := a.fileSet.Position(cg.End())
		lines := a.directives[pos.Filename]
		if lines == nil {
			lines = map[int]routeDirectives{}
			a.directives[pos.Filename] = lines
		}
		lines[pos.Line] = rd
	}
}

// parseDirectiveGroup folds every Directive line in one comment group into a
// single routeDirectives value. found is false when the group carries none.
func (a *ProjectAnalyzer) parseDirectiveGroup(cg *ast.CommentGroup) (rd routeDirectives, found bool) {
	for _, c := range cg.List {
		name, args, ok := parseDirective(c.Text)
		if !ok {
			continue
		}
		found = true
		a.applyDirective(&rd, name, args, c.Pos())
	}
	return rd, found
}

// applyDirective folds one parsed Directive into rd, diagnosing an unknown name
// or arguments that the name does not accept.
func (a *ProjectAnalyzer) applyDirective(rd *routeDirectives, name, args string, pos token.Pos) {
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
	}
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

// directivesAt returns the Directives attached to the registration call whose
// first token is at pos — i.e. those in a comment group ending on the line
// directly above it. The zero value is returned when there are none.
func (a *ProjectAnalyzer) directivesAt(pos token.Pos) routeDirectives {
	p := a.fileSet.Position(pos)
	return a.directives[p.Filename][p.Line-1]
}

// applyRouteDirectives stamps the Directives attached to the registration call
// at pos onto the route: `public` marks it auth-free, `errors` contributes the
// declared extra error statuses the generator unions with its baseline.
func (a *ProjectAnalyzer) applyRouteDirectives(route *models.Route, pos token.Pos) {
	rd := a.directivesAt(pos)
	if rd.public {
		route.Public = true
	}
	if len(rd.errorStatuses) > 0 {
		// Sorted and deduplicated so `404,404` (or two errors directives in one
		// group) declares 404 once, and the stamped order is deterministic.
		codes := slices.Clone(rd.errorStatuses)
		slices.Sort(codes)
		route.ErrorStatuses = slices.Compact(codes)
	}
}

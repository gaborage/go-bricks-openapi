package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// marshalerModSrc is a module whose types are Marshaler types (#111): DECLS
// adds declarations, ROUTES registrations and HANDLERS handlers.
const marshalerModSrc = `package svc

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

var _ uuid.UUID

type Level int

func (l Level) MarshalText() ([]byte, error) { return nil, nil }
func (l *Level) UnmarshalText(b []byte) error { return nil }

type MoneyT struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m MoneyT) MarshalText() ([]byte, error) { return nil, nil }
func (m *MoneyT) UnmarshalText(b []byte) error { return nil }

type MoneyF struct {
	A int ` + "`json:\"a\", validate:\"min=1\"`" + `
	P uintptr
}

func (m MoneyF) MarshalText() ([]byte, error) { return nil, nil }
func (m *MoneyF) UnmarshalText(b []byte) error { return nil }

type WrapLevel struct {
	Level
	Note string ` + "`json:\"note\"`" + `
}

type WithID struct {
	uuid.UUID
	Note string ` + "`json:\"note\"`" + `
}

type Money struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

type CustomBody struct {
	ID   string ` + "`param:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func (c *CustomBody) UnmarshalJSON(b []byte) error { return nil }

type Defaults struct{}

func (d *Defaults) UnmarshalJSON(b []byte) error { return nil }

type ListReq struct {
	Defaults
	Page int    ` + "`query:\"page\"`" + `
	ID   string ` + "`param:\"id\"`" + `
}

DECLS

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
ROUTES}

HANDLERS`

// textOut holds a field of each text Marshaler type, in pointer and slice
// forms too.
const textOut = `type Out struct {
	L   Level      ` + "`json:\"l\"`" + `
	LP  *Level     ` + "`json:\"lp\"`" + `
	LS  []Level    ` + "`json:\"ls\"`" + `
	M   MoneyT     ` + "`json:\"m\"`" + `
	MP  *MoneyT    ` + "`json:\"mp\"`" + `
	MS  []MoneyT   ` + "`json:\"ms\"`" + `
	F   MoneyF     ` + "`json:\"f\"`" + `
	W   WrapLevel  ` + "`json:\"w\"`" + `
	WS  []WrapLevel ` + "`json:\"ws\"`" + `
	ID  WithID     ` + "`json:\"id\"`" + `
	IDP *WithID    ` + "`json:\"idp\"`" + `
}
`

// marshalerRoute is one route of a marshalerModSrc project: its method, path,
// request type ("" for none) and response payload.
type marshalerRoute struct{ method, path, req, payload string }

// writeMarshalerProject writes marshalerModSrc with decls and routes as a
// project and returns its root.
func writeMarshalerProject(t *testing.T, decls string, routes ...marshalerRoute) string {
	t.Helper()
	var reg, handlers strings.Builder
	for i, r := range routes {
		fmt.Fprintf(&reg, "\tserver.%s(hr, r, %q, m.h%d)\n", r.method, r.path, i)
		req := ""
		if r.req != "" {
			req = "req " + r.req + ", "
		}
		fmt.Fprintf(&handlers, "func (m *Module) h%d(%sctx server.HandlerContext) (server.Result[%s], server.IAPIError) {\n"+
			"\treturn server.Result[%s]{}, nil\n}\n\n", i, req, r.payload, r.payload)
	}
	src := strings.NewReplacer("DECLS", decls, "ROUTES", reg.String(), "HANDLERS", handlers.String()).Replace(marshalerModSrc)
	return writeProject(t, "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks "+minGoBricksVer+"\n", src)
}

// textRoutes are the routes whose payloads and request are all text types.
var textRoutes = []marshalerRoute{
	{"GET", "/out", "", "Out"},
	{"GET", "/money-t", "", "MoneyT"},
	{"GET", "/money-f", "", "MoneyF"},
	{"GET", "/level", "", "Level"},
	{"GET", "/levels", "", "[]Level"},
	{"POST", "/money-t", "MoneyT", "Level"},
}

// runMarshalerGenerate runs generate on dir and returns its stdout, output
// path and error.
func runMarshalerGenerate(t *testing.T, dir string, strict, validate bool) (stdout, out string, runErr error) {
	t.Helper()
	out = filepath.Join(t.TempDir(), outputFileName)
	stdout = testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: strict, Validate: validate})
	})
	return stdout, out, runErr
}

// analyzerWarnings analyzes dir and returns its warnings (generate prints
// them to stderr, which tests do not capture).
func analyzerWarnings(t *testing.T, dir string) []string {
	t.Helper()
	a := analyzer.New(dir)
	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	return a.Warnings(t.Context())
}

// readMarshalerSpec reads the spec generate wrote.
func readMarshalerSpec(t *testing.T, out string) OpenAPISpec {
	t.Helper()
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))
	return spec
}

// TestRunGenerateTextMarshalersStrictClean pins that text Marshaler types
// raise no warning, so --strict --validate passes; MoneyF's own malformed tag
// and uintptr field are never read (Impact item 7).
func TestRunGenerateTextMarshalersStrictClean(t *testing.T) {
	dir := writeMarshalerProject(t, textOut, textRoutes...)
	stdout, _, runErr := runMarshalerGenerate(t, dir, true, true)
	require.NoError(t, runErr, stdout)
	assert.Equal(t, "Warnings: 0", warningsSummary(t, stdout))
}

// TestRunGenerateStructMarshalerFailsStrict pins that one struct Marshaler
// field fails --strict with its warning printed.
func TestRunGenerateStructMarshalerFailsStrict(t *testing.T) {
	decls := strings.Replace(textOut, "}\n", "\tMoney Money `json:\"money\"`\n}\n", 1)
	dir := writeMarshalerProject(t, decls, textRoutes...)
	stdout, out, runErr := runMarshalerGenerate(t, dir, true, false)
	require.Error(t, runErr)
	assert.Equal(t, "Warnings: 1", warningsSummary(t, stdout))
	assert.NoFileExists(t, out)
	warnings := analyzerWarnings(t, dir)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "Money has its own MarshalJSON method")
}

// TestDoctorClassifiesMarshalerTypes pins doctor's verdict: a text payload or
// request is typed, a non-text one is not, and a params-only Marshaler
// request stays typed.
func TestDoctorClassifiesMarshalerTypes(t *testing.T) {
	dir := writeMarshalerProject(t, "",
		marshalerRoute{"GET", "/money", "", "Money"},
		marshalerRoute{"GET", "/money-t", "", "MoneyT"},
		marshalerRoute{"POST", "/custom/:id", "CustomBody", "Money"},
		marshalerRoute{"POST", "/money-t", "MoneyT", "Money"},
		marshalerRoute{"GET", "/items/:id", "ListReq", "Money"})
	project, err := analyzer.New(dir).AnalyzeProject()
	require.NoError(t, err)
	stats := calculateProjectStats(project)
	got := append([]string{}, stats.UntypedRoutes...)
	sort.Strings(got)
	assert.Equal(t, []string{"h0", "h2"}, got)
	assert.Equal(t, 2, stats.TypedRequestRoutes)
	assert.Equal(t, 1, stats.TypedResponseRoutes)
}

// edgeDecls are review round 1's edge rows: struct parameters of a Marshaler
// request, and a JOSE request and response.
const edgeDecls = `type Filter struct {
	A string ` + "`query:\"a\"`" + `
}

type Empty struct{}

type CmdBody struct {
	F    Filter ` + "`query:\"f\"`" + `
	E    Empty  ` + "`query:\"e\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func (c *CmdBody) UnmarshalJSON(b []byte) error { return nil }

type JoseReq struct {
	_    struct{} ` + "`jose:\"decrypt=k1,verify=k2\"`" + `
	Name string   ` + "`json:\"name\"`" + `
}

func (j *JoseReq) UnmarshalJSON(b []byte) error { return nil }

type JoseResp struct {
	_ struct{} ` + "`jose:\"sign=k1,encrypt=k2\"`" + `
	A int64    ` + "`json:\"a\"`" + `
}

func (j JoseResp) MarshalJSON() ([]byte, error) { return nil, nil }
`

// TestRunGenerateStructMarshalerEdgeRoutesValidate pins that the edge rows
// validate (no $ref dangles), JOSE stays application/jose, and a params-only
// Marshaler request has no body and passes --strict.
func TestRunGenerateStructMarshalerEdgeRoutesValidate(t *testing.T) {
	items := marshalerRoute{"GET", "/items/:id", "ListReq", "Level"}
	dir := writeMarshalerProject(t, edgeDecls,
		marshalerRoute{"POST", "/cmd", "CmdBody", "Level"},
		marshalerRoute{"POST", "/jose", "JoseReq", "JoseResp"},
		items)
	stdout, out, runErr := runMarshalerGenerate(t, dir, false, true)
	require.NoError(t, runErr, stdout)
	spec := readMarshalerSpec(t, out)
	assert.Contains(t, digMap(t, spec.Paths, "/jose", "post", "requestBody", "content"), "application/jose")
	assert.NotContains(t, digMap(t, spec.Paths, "/items/{id}", "get"), "requestBody")
	schemas, ok := spec.Components["schemas"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, schemas, "Filter")
	assert.Contains(t, schemas, "Empty")

	dir = writeMarshalerProject(t, "", items)
	stdout, _, runErr = runMarshalerGenerate(t, dir, true, true)
	require.NoError(t, runErr, stdout)
	assert.Equal(t, "Warnings: 0", warningsSummary(t, stdout))
}

// TestRunGenerateZeroFieldMarshalerRequest pins review round 3's finding 1: a
// zero-field Marshaler request gets a body, so a non-text one fails --strict
// and a text one passes.
func TestRunGenerateZeroFieldMarshalerRequest(t *testing.T) {
	ping := "type Ping struct{}\n\nfunc (p *Ping) UnmarshalJSON(b []byte) error { return nil }\n"
	dir := writeMarshalerProject(t, ping, marshalerRoute{"GET", "/ping", "Ping", "Level"})
	stdout, _, runErr := runMarshalerGenerate(t, dir, true, false)
	require.Error(t, runErr)
	assert.Equal(t, "Warnings: 1", warningsSummary(t, stdout))
	warnings := analyzerWarnings(t, dir)
	require.Len(t, warnings, 1)
	assert.True(t, strings.HasPrefix(warnings[0], "request type Ping: "), warnings[0])
	stdout, out, runErr := runMarshalerGenerate(t, dir, false, false)
	require.NoError(t, runErr, stdout)
	schema := digMap(t, readMarshalerSpec(t, out).Paths, "/ping", "get", "requestBody", "content", "application/json", "schema")
	assert.Empty(t, schema)

	blank := "type Blank struct{}\n\nfunc (x Blank) MarshalText() ([]byte, error) { return nil, nil }\n" +
		"func (x *Blank) UnmarshalText(b []byte) error { return nil }\n"
	dir = writeMarshalerProject(t, blank, marshalerRoute{"POST", "/blank", "Blank", "Level"})
	stdout, out, runErr = runMarshalerGenerate(t, dir, true, true)
	require.NoError(t, runErr, stdout)
	assert.Equal(t, "Warnings: 0", warningsSummary(t, stdout))
	schema = digMap(t, readMarshalerSpec(t, out).Paths, "/blank", "post", "requestBody", "content", "application/json", "schema")
	assert.Equal(t, map[string]any{"type": "string"}, schema)
}

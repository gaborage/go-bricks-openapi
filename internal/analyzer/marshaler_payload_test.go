package analyzer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// mtRequestDecls are the request and JOSE types of the edge rows (#111):
// Appendix B's plus the params-only and JOSE variants pinned by unit tests.
const mtRequestDecls = `
type Cents int64

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

type Filter struct {
	A string ` + "`query:\"a\"`" + `
}

type Empty struct{}

type CmdBody struct {
	F    Filter ` + "`query:\"f\"`" + `
	E    Empty  ` + "`query:\"e\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func (c *CmdBody) UnmarshalJSON(b []byte) error { return nil }

type Defaults struct{}

func (d *Defaults) UnmarshalJSON(b []byte) error { return nil }

type ListReq struct {
	Defaults
	Page int    ` + "`query:\"page\"`" + `
	ID   string ` + "`param:\"id\"`" + `
}

type JoseParams struct {
	_  struct{} ` + "`jose:\"decrypt=k1,verify=k2\"`" + `
	ID string   ` + "`param:\"id\"`" + `
}

func (j *JoseParams) UnmarshalJSON(b []byte) error { return nil }

type TextParams struct {
	ID string ` + "`param:\"id\"`" + `
}

func (x TextParams) MarshalText() ([]byte, error) { return nil, nil }
func (x *TextParams) UnmarshalText(b []byte) error { return nil }

type JoseText struct {
	_ struct{} ` + "`jose:\"decrypt=k1,verify=k2\"`" + `
}

func (x JoseText) MarshalText() ([]byte, error) { return nil, nil }
func (x *JoseText) UnmarshalText(b []byte) error { return nil }
`

const (
	mtCtx      = "(ctx server.HandlerContext) "
	mtLevelRes = "(server.Result[Level], server.IAPIError)"
)

// mtReq is a handler signature taking a request of type req.
func mtReq(req string) string {
	return "(req " + req + ", ctx server.HandlerContext) " + mtLevelRes
}

// mtRes is a handler signature returning payload.
func mtRes(payload string) string {
	return mtCtx + "(server.Result[" + payload + "], server.IAPIError)"
}

// TestMarshalerPayloadRows pins Marshaler payloads: a non-text one is {}
// with one warning and no component, a text one is typed silently.
func TestMarshalerPayloadRows(t *testing.T) {
	a, routes := analyzeMT(t, mtModule("",
		"GET /money "+mtRes(mtMoney), "GET /money-ptr "+mtRes("*Money"), "GET /moneys "+mtRes("[]Money"),
		"GET /stamped "+mtRes("Stamped"), "GET /money-t "+mtRes(mtMoneyT), "GET /wrap-level "+mtRes(mtWrapLevel),
		"GET /level "+mtRes(mtLevel), "GET /levels "+mtRes("[]Level")))
	untyped := map[string]string{
		"/money":     "marshal:Money(unknown)",
		"/money-ptr": "*marshal:Money(unknown)",
		"/moneys":    "[]marshal:Money(unknown)",
		"/stamped":   "marshal:Stamped(unknown)",
	}
	typed := map[string]string{
		"/money-t": "text:MoneyT", "/wrap-level": "text:WrapLevel", "/level": "text:Level", "/levels": "[]text:Level",
	}
	for path, want := range mergeRows(untyped, typed) {
		resp := routeByPath(t, routes, path).Response
		require.NotNil(t, resp, path)
		require.NotNil(t, resp.Resolution, path)
		assert.Equal(t, want, renderShape(*resp.Resolution), path)
		assert.Empty(t, resp.Name, path)
		_, isTyped := typed[path]
		assert.Equal(t, isTyped, IsTypedPayload(resp), path)
	}
	assert.ElementsMatch(t, []string{
		fmt.Sprintf(marshalerPayloadWarning, mtMoney, mtMoney, methodMarshalJSON),
		fmt.Sprintf(marshalerPayloadWarning, "*Money", mtMoney, methodMarshalJSON),
		fmt.Sprintf(marshalerPayloadWarning, "[]Money", mtMoney, methodMarshalJSON),
		fmt.Sprintf(marshalerPromotedPayloadWarning, "Stamped", "Stamped", methodMarshalJSON, models.WellKnownTimeTime),
	}, a.warnings)
	assert.Empty(t, a.typeRegistry)
}

// mergeRows returns the union of two path -> rendering maps.
func mergeRows(x, y map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range x {
		out[k] = v
	}
	for k, v := range y {
		out[k] = v
	}
	return out
}

// TestMarshalerRequest pins struct Marshaler requests: an untyped body with
// one warning and only parameters kept, a text one a string body; non-struct
// requests keep #127's warning.
func TestMarshalerRequest(t *testing.T) {
	a, routes := analyzeMT(t, mtModule(mtRequestDecls,
		"POST /custom/:id "+mtReq("CustomBody"), "POST /wrapper "+mtReq("*Wrapper"), "POST /money-t "+mtReq(mtMoneyT),
		"POST /price "+mtReq(mtPrice), "POST /level "+mtReq(mtLevel), "POST /cents "+mtReq("Cents")))
	custom := routeByPath(t, routes, "/custom/{id}").Request
	require.NotNil(t, custom)
	assert.Empty(t, custom.Name)
	require.NotNil(t, custom.Resolution)
	assert.Equal(t, models.ShapeMarshaler, custom.Resolution.Kind)
	require.Len(t, custom.Fields, 1)
	assert.Equal(t, "ID", custom.Fields[0].Name)
	assert.Equal(t, "path", custom.Fields[0].ParamType)
	assert.False(t, IsTypedPayload(custom))

	for path, want := range map[string]models.ShapeKind{
		"/wrapper": models.ShapeMarshaler, "/money-t": models.ShapeText, "/price": models.ShapeMarshaler,
	} {
		req := routeByPath(t, routes, path).Request
		if assert.NotNil(t, req.Resolution, path) {
			assert.Equal(t, want, req.Resolution.Kind, path)
		}
		assert.Equal(t, want == models.ShapeText, IsTypedPayload(req), path)
	}
	for _, path := range []string{"/level", "/cents"} {
		req := routeByPath(t, routes, path).Request
		assert.Nil(t, req.Resolution, path)
		assert.Empty(t, req.Name, path)
	}
	assert.ElementsMatch(t, []string{
		fmt.Sprintf(marshalerRequestWarning, "CustomBody", "CustomBody", methodUnmarshalJSON),
		fmt.Sprintf(marshalerPromotedRequestWarning, "*Wrapper", "Wrapper", methodMarshalText, mtStatusV),
		fmt.Sprintf(marshalerRequestWarning, mtPrice, mtPrice, methodMarshalJSON),
		fmt.Sprintf(nonStructRequestWarning, mtLevel),
		fmt.Sprintf(nonStructRequestWarning, "Cents"),
	}, a.warnings)
	assert.Empty(t, a.typeRegistry)
}

// TestMarshalerRequestEdgeRows pins JOSE, params-only and struct-parameter
// edge rows (review round 1).
func TestMarshalerRequestEdgeRows(t *testing.T) {
	a, routes := analyzeMT(t, mtModule(mtRequestDecls,
		"POST /jose (req JoseReq, ctx server.HandlerContext) (server.Result[JoseResp], server.IAPIError)",
		"GET /jose-ptr "+mtRes("*JoseResp"), "GET /jose-slice "+mtRes("[]JoseResp"),
		"POST /jose-params/:id "+mtReq("JoseParams"), "GET /items/:id "+mtReq(mtListReq),
		"POST /text-params/:id "+mtReq("TextParams"), "POST /jose-text "+mtReq("JoseText"), "POST /cmd "+mtReq("CmdBody")))
	jose := routeByPath(t, routes, "/jose")
	for _, ti := range []*models.TypeInfo{jose.Request, jose.Response, routeByPath(t, routes, "/jose-ptr").Response,
		routeByPath(t, routes, "/jose-slice").Response} {
		assert.True(t, ti.JOSE)
		assert.Empty(t, ti.Name)
	}
	joseParams := routeByPath(t, routes, "/jose-params/{id}").Request
	assert.True(t, joseParams.JOSE)
	assert.NotNil(t, joseParams.Resolution)
	require.Len(t, joseParams.Fields, 1)
	assert.Equal(t, "ID", joseParams.Fields[0].Name)

	for _, path := range []string{"/items/{id}", "/text-params/{id}"} {
		req := routeByPath(t, routes, path).Request
		assert.NotEmpty(t, req.Name, path)
		assert.Nil(t, req.Resolution, path)
		assert.True(t, IsTypedPayload(req), path)
	}
	list := routeByPath(t, routes, "/items/{id}").Request
	assert.Equal(t, mtListReq, list.Name)
	require.Len(t, list.Fields, 2)
	assert.Equal(t, []string{"Page", "ID"}, []string{list.Fields[0].Name, list.Fields[1].Name})

	joseText := routeByPath(t, routes, "/jose-text").Request
	assert.True(t, joseText.JOSE)
	require.NotNil(t, joseText.Resolution)
	assert.Equal(t, models.ShapeText, joseText.Resolution.Kind)
	assert.True(t, IsTypedPayload(joseText))

	cmd := routeByPath(t, routes, "/cmd").Request
	require.Len(t, cmd.Fields, 2)
	assert.Equal(t, "$Filter", renderShape(cmd.Fields[0].ResolvedShape()))
	assert.Equal(t, "$Empty", renderShape(cmd.Fields[1].ResolvedShape()))

	assert.ElementsMatch(t, []string{mtFilter, mtEmpty}, mapKeys(a.typeRegistry))
	assert.ElementsMatch(t, []string{
		fmt.Sprintf(marshalerRequestWarning, "JoseReq", "JoseReq", methodUnmarshalJSON),
		fmt.Sprintf(marshalerPayloadWarning, "JoseResp", "JoseResp", methodMarshalJSON),
		fmt.Sprintf(marshalerPayloadWarning, "*JoseResp", "JoseResp", methodMarshalJSON),
		fmt.Sprintf(marshalerPayloadWarning, "[]JoseResp", "JoseResp", methodMarshalJSON),
		fmt.Sprintf(marshalerRequestWarning, "JoseParams", "JoseParams", methodUnmarshalJSON),
		fmt.Sprintf(marshalerRequestWarning, "CmdBody", "CmdBody", methodUnmarshalJSON),
	}, a.warnings)
}

// mapKeys returns m's keys.
func mapKeys(m map[string]*models.TypeInfo) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestZeroFieldMarshalerRequest pins review round 3's finding 1: a zero-field
// struct Marshaler request is not params-only, so it gets a body.
func TestZeroFieldMarshalerRequest(t *testing.T) {
	decls := `
type Ping struct{}

func (p *Ping) UnmarshalJSON(b []byte) error { return nil }

type Blank struct{}

func (x Blank) MarshalText() ([]byte, error) { return nil, nil }
func (x *Blank) UnmarshalText(b []byte) error { return nil }

type Hidden struct {
	ID     string ` + "`param:\"id\"`" + `
	secret string
}

func (h *Hidden) UnmarshalJSON(b []byte) error { return nil }

type Plain struct{}
`
	a, routes := analyzeMT(t, mtModule(decls, "GET /ping "+mtReq("Ping"), "POST /blank "+mtReq("Blank"),
		"GET /hidden/:id "+mtReq("Hidden"), "POST /plain "+mtReq("Plain")))
	ping := routeByPath(t, routes, "/ping").Request
	assert.Empty(t, ping.Name)
	require.NotNil(t, ping.Resolution)
	assert.Equal(t, models.ShapeMarshaler, ping.Resolution.Kind)
	assert.Empty(t, ping.Fields)
	assert.False(t, IsTypedPayload(ping))

	blank := routeByPath(t, routes, "/blank").Request
	assert.Empty(t, blank.Name)
	require.NotNil(t, blank.Resolution)
	assert.Equal(t, models.ShapeText, blank.Resolution.Kind)
	assert.True(t, IsTypedPayload(blank))

	hidden := routeByPath(t, routes, "/hidden/{id}").Request
	assert.Equal(t, "Hidden", hidden.Name)
	assert.Nil(t, hidden.Resolution)
	require.Len(t, hidden.Fields, 1)
	assert.Equal(t, "ID", hidden.Fields[0].Name)

	plain := routeByPath(t, routes, "/plain").Request
	assert.Equal(t, "Plain", plain.Name)
	assert.Contains(t, a.typeRegistry, "Plain")

	assert.Equal(t, []string{fmt.Sprintf(marshalerRequestWarning, "Ping", "Ping", methodUnmarshalJSON)}, a.warnings)
}

// TestMarshalerRequestTagWarnings pins review round 2's residual: a struct
// Marshaler request's fields are still extracted, so a malformed tag on any
// of them warns; reached only as a field, the same struct warns nothing.
func TestMarshalerRequestTagWarnings(t *testing.T) {
	decls := `
type Local struct {
	ID   string ` + "`param:\"id\"`" + `
	Name string ` + "`json:\"name\", validate:\"x\"`" + `
	Q    string ` + "`json:\"q\", query:\"q\"`" + `
}

func (x Local) MarshalText() ([]byte, error) { return nil, nil }
func (x *Local) UnmarshalText(b []byte) error { return nil }
`
	a, routes := analyzeMT(t, mtModule(decls, "POST /local/:id "+mtReq("Local")))
	req := routeByPath(t, routes, "/local/{id}").Request
	assert.Empty(t, req.Name)
	require.NotNil(t, req.Resolution)
	assert.Equal(t, models.ShapeText, req.Resolution.Kind)
	require.Len(t, req.Fields, 1)
	assert.Equal(t, "ID", req.Fields[0].Name)
	tags := tagWarnings(a.warnings)
	assert.Len(t, a.warnings, 2, "no request warning: %v", a.warnings)
	if assert.Len(t, tags, 2) {
		joined := strings.Join(tags, "\n")
		assert.Contains(t, joined, "field Name at")
		assert.Contains(t, joined, "validate")
		assert.Contains(t, joined, "field Q at")
		assert.Contains(t, joined, "query")
	}

	out := decls + "\ntype Out struct {\n\tL Local `json:\"l\"`\n}\n"
	a, _ = analyzeMT(t, mtModule(out, "GET /out "+mtRes("Out")))
	assert.Empty(t, a.warnings)
}

// TestStructMarshalerJOSE pins the jose flag read at the Marshaler gate.
func TestStructMarshalerJOSE(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)
	file, path := parseInDir(t, a, dir, "t.go", "package p\n\n"+
		"type J struct {\n\t_ struct{} `jose:\"sign=k1\"`\n}\n\ntype P struct{ N int }\n\ntype L int\n")
	assert.True(t, a.structMarshalerJOSE(&models.TypeInfo{Name: "J"}, file, path))
	assert.False(t, a.structMarshalerJOSE(&models.TypeInfo{Name: "P"}, file, path))
	assert.False(t, a.structMarshalerJOSE(&models.TypeInfo{Name: "L"}, file, path))
}

// TestWarnMarshalerRequestEveryKind drives warnMarshalerRequest with each
// fallback kind it reads.
func TestWarnMarshalerRequestEveryKind(t *testing.T) {
	a := New("")
	a.warnMarshalerRequest("R", fieldFallback{kind: fallbackMarshaler, typeName: "R", detail: methodUnmarshalJSON})
	a.warnMarshalerRequest("*W", fieldFallback{kind: fallbackMarshalerPromoted, typeName: "W", detail: methodMarshalText, via: mtStatusV})
	a.warnMarshalerRequest("T", fieldFallback{})
	assert.Equal(t, []string{
		fmt.Sprintf(marshalerRequestWarning, "R", "R", methodUnmarshalJSON),
		fmt.Sprintf(marshalerPromotedRequestWarning, "*W", "W", methodMarshalText, mtStatusV),
	}, a.warnings)
}

// TestNamedShapeAndMarshalerNamed pins how a TypeInfo's name is spelled for
// resolution, and that a nameless one is never a Marshaler type.
func TestNamedShapeAndMarshalerNamed(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)
	file, path := parseInDir(t, a, dir, "t.go", "package p\n")
	assert.Equal(t, "T", namedShape(&models.TypeInfo{Name: "T"}, file).Name)
	assert.Equal(t, "T", namedShape(&models.TypeInfo{Name: "T", Package: "p"}, file).Name)
	assert.Equal(t, "q.T", namedShape(&models.TypeInfo{Name: "T", Package: "q"}, file).Name)
	_, _, ok := a.marshalerNamed(&models.TypeInfo{}, file, path)
	assert.False(t, ok)
}

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
	"github.com/gaborage/go-bricks-openapi/internal/models"
	"github.com/gaborage/go-bricks-openapi/internal/specvalidate"
	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// Schemas the fixed-size array rows (#98) expect, as the YAML decoder reads
// them back. byteItem is exactly what a bare byte field emits.
var (
	byteItem     = map[string]any{"type": "integer", "format": "int32", "minimum": 0}
	byteArray    = map[string]any{"type": "array", "items": byteItem}
	int64Array   = map[string]any{"type": "array", "items": map[string]any{"type": "integer", "format": "int64"}}
	base64String = map[string]any{"type": "string", "format": "byte"}
	addressRef   = map[string]any{"$ref": "#/components/schemas/Address"}
	addressArray = map[string]any{"type": "array", "items": addressRef}
	// boundedArray is a [4]byte or [4]uint16 field under
	// validate:"min=1,max=4,dive,max=200": cardinality on the array, the dive
	// bound on its items, over the unsigned floor.
	boundedArray = map[string]any{
		"type": "array", "minItems": 1, "maxItems": 4,
		"items": map[string]any{"type": "integer", "format": "int32", "minimum": 0, "maximum": 200},
	}
)

// fixedArrayField is one CreateReq field: its Go type and extra struct tags,
// and the exact property schema it must emit. Rows marked "must not move" pin
// the schema main already emitted, not the field's slice form — #97 will make
// []Flag base64 while [4]Flag stays an integer array.
type fixedArrayField struct {
	json   string
	goType string
	tags   string
	want   map[string]any
}

var fixedArrayFields = []fixedArrayField{
	{json: "digest", goType: "[32]byte", want: byteArray},
	{json: "key", goType: "[4]byte", want: byteArray},
	{json: "octets", goType: "[4]uint8", want: byteArray},
	{json: "empty", goType: "[0]byte", want: byteArray},
	{json: "sized", goType: "[N]byte", want: byteArray},
	{json: "maybeKey", goType: "*[4]byte", want: byteArray}, // no nullable, like *[4]int
	{json: "keys", goType: "[][4]byte", want: map[string]any{"type": "array", "items": byteArray}},
	{json: "keyPairs", goType: "[2][4]byte", want: map[string]any{"type": "array", "items": byteArray}},
	{json: "keyMap", goType: "map[string][4]byte", want: map[string]any{"type": "object", "additionalProperties": byteArray}},
	{json: "bounded", goType: "[4]byte", tags: `validate:"min=1,max=4,dive,max=200"`, want: boundedArray},
	{json: "boundedWide", goType: "[4]uint16", tags: `validate:"min=1,max=4,dive,max=200"`, want: boundedArray},
	{json: "sample", goType: "[4]byte", tags: `example:"AQIDBA=="`, want: byteArray}, // an array takes no scalar example
	{json: "sampleWide", goType: "[4]uint16", tags: `example:"AQIDBA=="`, want: byteArray},
	// Must not move.
	{json: "flags", goType: "[4]Flag", want: byteArray},
	{json: "bytePtrs", goType: "[4]*byte", want: byteArray},
	{json: "wide", goType: "[4]uint16", want: byteArray},
	{json: "ints", goType: "[3]int", want: int64Array},
	{json: "maybeInts", goType: "*[4]int", want: int64Array},
	{json: "blobs", goType: "[2][]byte", want: map[string]any{"type": "array", "items": base64String}},
	{json: "addrs", goType: "[2]Address", want: addressArray},
	{json: "addrPtrs", goType: "[2]*Address", want: addressArray},
	{json: "maybeAddrs", goType: "*[2]Address", want: addressArray},
	{json: "addrMap", goType: "map[string][2]Address", want: map[string]any{"type": "object", "additionalProperties": addressArray}},
	{json: "amounts", goType: "[3]Cents", tags: `validate:"dive,min=1"`, want: map[string]any{
		"type": "array", "items": map[string]any{"type": "integer", "format": "int64", "minimum": 1},
	}},
	{json: "names", goType: "[2]string", tags: `validate:"dive,min=2"`, want: map[string]any{
		"type": "array", "items": map[string]any{"type": "string", "minLength": 2},
	}},
	{json: "handles", goType: "[2]uintptr", want: map[string]any{"type": "array", "items": map[string]any{"type": "object"}}},
	{json: "raw", goType: "[]byte", want: base64String},
	{json: "maybeRaw", goType: "*[]byte", want: base64String},
	{json: "rawOctets", goType: "[]uint8", want: base64String},
}

// fixedArrayModSrc declares the fixedArrayFields rows as CreateReq (FIELDS is
// spliced in) and a [4]byte query parameter on ListReq.
const fixedArrayModSrc = `package svc

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

const N = 4

type Flag byte

type Cents int64

type Address struct {
	Street string ` + "`json:\"street\"`" + `
}

type ListReq struct {
	Prefix [4]byte ` + "`query:\"prefix\"`" + `
}

type CreateReq struct {
FIELDS}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/keys", m.list, server.WithTags("svc"))
	server.POST(hr, r, "/keys", m.create, server.WithTags("svc"))
}

func (m *Module) list(req ListReq, ctx server.HandlerContext) (server.Result[Address], server.IAPIError) {
	return out, nil
}

func (m *Module) create(req CreateReq, ctx server.HandlerContext) (server.Result[Address], server.IAPIError) {
	return out, nil
}
`

// TestRunGenerateFixedArrayFields pins every field row of #98 end to end, from
// the decoded Shape to the emitted property: [N]byte/[N]uint8 in any position
// is the integer array encoding/json writes, never a base64 string; a [4]byte
// takes the validate and example tags exactly as a [4]uint16 does; a query
// parameter follows the field rule; and every control keeps the schema it had.
// The only warning is the uintptr one the [2]uintptr control always raised.
func TestRunGenerateFixedArrayFields(t *testing.T) {
	var fields strings.Builder
	for i, f := range fixedArrayFields {
		tag := `json:"` + f.json + `"`
		if f.tags != "" {
			tag += " " + f.tags
		}
		fmt.Fprintf(&fields, "\tF%d %s `%s`\n", i, f.goType, tag)
	}
	src := strings.Replace(fixedArrayModSrc, "FIELDS", fields.String(), 1)
	dir := writeProject(t, "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks "+minGoBricksVer+"\n", src)

	a := analyzer.New(dir)
	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "is a uintptr")

	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: out})
	})
	require.NoError(t, runErr)
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	require.NoError(t, specvalidate.Validate(t.Context(), content))
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))

	props := digMap(t, spec.Components, "schemas", "CreateReq", "properties")
	for _, f := range fixedArrayFields {
		assert.Equal(t, f.want, props[f.json], "%s %s", f.goType, f.tags)
	}

	params, ok := digMap(t, spec.Paths, "/keys", "get")["parameters"].([]any)
	require.True(t, ok, "GET /keys has parameters")
	var prefix any
	for _, p := range params {
		if pm, isMap := p.(map[string]any); isMap && pm["name"] == "prefix" {
			prefix = pm["schema"]
		}
	}
	assert.Equal(t, byteArray, prefix, "a [4]byte query parameter takes the field rule, with no special case")
}

// TestRunGenerateFixedArrayPayloads pins every payload row of #98: a
// fixed-size array type argument of server.Result / server.ResultWithMeta, and
// the same array as a bare return (routed through the same payload helpers),
// documents its data as the array encoding/json writes, is typed for doctor,
// raises no warning and emits no Time component for a well-known element.
func TestRunGenerateFixedArrayPayloads(t *testing.T) {
	cases := []struct {
		payload string
		want    map[string]any
	}{
		{payload: "[4]byte", want: byteArray},
		{payload: "[32]uint8", want: byteArray},
		{payload: "[4]*byte", want: byteArray},
		// Must not move.
		{payload: "[3]int", want: int64Array},
		{payload: "[2]Address", want: addressArray},
		{payload: "[2]*Address", want: addressArray},
		{payload: "[2]time.Time", want: map[string]any{"type": "array", "items": map[string]any{"type": "string", "format": "date-time"}}},
		{payload: "[]byte", want: base64String},
	}
	for _, c := range cases {
		t.Run(c.payload, func(t *testing.T) {
			wrapped := bareReturnOutcome(t, "server.Result["+c.payload+"]", false)
			assert.Equal(t, c.want, wrapped.Schema)
			assert.True(t, wrapped.Valid)
			assert.True(t, wrapped.TypedResponse, "doctor counts server.Result[%s] typed", c.payload)
			assert.False(t, wrapped.StrictFails)
			assert.Equal(t, "Warnings: 0", wrapped.Summary)
			assert.NotContains(t, wrapped.Components, "Time")

			assert.Equal(t, wrapped, bareReturnOutcome(t, c.payload, false), "a bare %s return is documented like server.Result[%s]", c.payload, c.payload)
			assert.Equal(t, wrapped, bareReturnOutcome(t, "server.ResultWithMeta["+c.payload+"]", false), "ResultWithMeta behaves like Result")
		})
	}
}

// TestIsTypedPayloadFixedArray pins doctor's verdict on the array payloads of
// #98: classified by the element, exactly as the slice form is.
func TestIsTypedPayloadFixedArray(t *testing.T) {
	assert.True(t, isTypedPayload(&models.TypeInfo{Shape: ptrTo(arrayOf(prim("byte")))}), "server.Result[[4]byte]")
	assert.True(t, isTypedPayload(&models.TypeInfo{Shape: ptrTo(arrayOf(prim("int")))}), "server.Result[[3]int]")
	assert.True(t, isTypedPayload(&models.TypeInfo{Name: "Address", Shape: ptrTo(arrayOf(named("Address")))}), "server.Result[[2]Address]")
}

package analyzer

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cancelledDecls are the local text and JSON types whose methods cancel a
// well-known Marshaler embed's promoted ones at the same depth.
const cancelledDecls = `
type Level int

func (Level) MarshalText() ([]byte, error)  { return nil, nil }
func (*Level) UnmarshalText(b []byte) error { return nil }

type Doc []byte

func (Doc) MarshalJSON() ([]byte, error)  { return nil, nil }
func (*Doc) UnmarshalJSON(b []byte) error { return nil }

type Stamp string

func (Stamp) MarshalJSON() ([]byte, error)  { return nil, nil }
func (*Stamp) UnmarshalJSON(b []byte) error { return nil }
func (Stamp) MarshalText() ([]byte, error)  { return nil, nil }
func (*Stamp) UnmarshalText(b []byte) error { return nil }
func (Stamp) AppendText(b []byte) ([]byte, error) { return b, nil }

type Count int

func (Count) MarshalJSONTo(enc *jsontext.Encoder) error    { return nil }
func (*Count) UnmarshalJSONFrom(dec *jsontext.Decoder) error { return nil }

type lu = uuid.UUID
`

// TestCancelledWellKnownMarshalerEmbed pins issue 163: when a sibling embed
// at the same depth cancels a well-known Marshaler embed's promoted methods,
// the embedding struct is no Marshaler and encoding/json writes the embed as
// a field. A non-struct one (uuid.UUID, *uuid.UUID, json.RawMessage, net.IP,
// json.Number) becomes one property keyed by its type name, typed as a field
// of the written type; a well-known struct (time.Time, netip.Addr), with no
// exported field, adds nothing. None of them warns; the local JSON
// Marshaler siblings (Doc, Stamp, Count) warn as any such field does.
func TestCancelledWellKnownMarshalerEmbed(t *testing.T) {
	rows := []embedRow{
		{"CUUID", "uuid.UUID\n\tLevel", map[string]string{"UUID": "uuid.UUID", "Level": "text:Level"}},
		{"CPtrUUID", "*uuid.UUID\n\tLevel", map[string]string{"UUID": "*uuid.UUID", "Level": "text:Level"}},
		{"CAlias", "uuid.UUID\n\tlu", map[string]string{"UUID": "uuid.UUID"}},
		{"CRaw", "json.RawMessage\n\tDoc", map[string]string{"RawMessage": "json.RawMessage", "Doc": "marshal:Doc([]byte)"}},
		{"CTime", "time.Time\n\tStamp", map[string]string{"Stamp": "marshal:Stamp(string)"}},
		{"CNet", "net.IP\n\tnetip.Addr", map[string]string{"IP": "net.IP"}},
		{"CNum", "json.Number\n\tCount", map[string]string{"Number": "json.Number", "Count": "marshal:Count(int)"}},
	}
	var decls, fields strings.Builder
	decls.WriteString(cancelledDecls)
	for _, r := range rows {
		fmt.Fprintf(&decls, "type %s struct {\n\t%s\n\tY int `json:\"y\"`\n}\n", r.typ, r.embed)
		fmt.Fprintf(&fields, "\t%s %s `json:\"%s\"`\n", r.typ, r.typ, strings.ToLower(r.typ))
	}
	imports := "\t\"encoding/json\"\n\t\"encoding/json/jsontext\"\n\t\"net\"\n\t\"net/netip\"\n\t\"time\"\n\t\"github.com/google/uuid\"\n"
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("mod", "module.go"): rowsModule(imports, decls.String(), fields.String()),
	})
	for _, r := range rows {
		want := map[string]string{"y": goTypeInt}
		for k, v := range r.want {
			want[k] = v
		}
		assert.Equal(t, want, renders(t, a.typeRegistry[r.typ]), r.typ)
	}
	assert.Empty(t, embedWarnings(a))
	all := a.Warnings(t.Context())
	assert.Len(t, all, 3, "only the Doc, Stamp and Count fields' own-method warnings: %v", all)
}

// TestMarshalerRequestEmbedStaysQuiet pins the other side of issue 163: a
// struct Marshaler request read only for its parameters (requestParamFields,
// under quietEmbeds) gains no property for its untagged uuid.UUID embed, so
// it stays params-only, with no body documented.
func TestMarshalerRequestEmbedStaysQuiet(t *testing.T) {
	_, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): embedRequestHead + `
type IDReq struct {
	uuid.UUID
	ID string ` + "`param:\"id\"`" + `
}

type Out struct {
	X int ` + "`json:\"x\"`" + `
}

func (m *Module) ids(req IDReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/ids/:id", m.ids)
}
`,
	})
	r := routeByPath(t, routes, "/ids/{id}")
	require.NotNil(t, r.Request)
	require.Len(t, r.Request.Fields, 1)
	assert.Equal(t, "id", r.Request.Fields[0].ParamName)
	assert.Equal(t, "IDReq", r.Request.Name, "params-only: the name is kept")
	assert.Nil(t, r.Request.Resolution, "params-only: no Marshaler body")
}

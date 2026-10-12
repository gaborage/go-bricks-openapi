package generator

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/models"
	"github.com/gaborage/go-bricks-openapi/internal/specvalidate"
)

// Issue 166: a field with no JSON name is keyed by its Go name, as
// encoding/json writes it, never with its first letter lowered.

// wireKeys is the keys encoding/json writes v under, sorted.
func wireKeys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var obj map[string]any
	require.NoError(t, json.Unmarshal(raw, &obj))
	return slices.Sorted(maps.Keys(obj))
}

// propertyKeys is a schema's property keys, sorted.
func propertyKeys(s *OpenAPISchema) []string {
	return slices.Sorted(maps.Keys(s.Properties))
}

// postRouteSchemas generates the spec of one POST route taking req and
// returning resp, requires it to validate, and returns its component schemas.
func postRouteSchemas(t *testing.T, req, resp *models.TypeInfo) map[string]*OpenAPISchema {
	t.Helper()
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	spec, err := gen.Generate(&models.Project{
		Name:    "KeysApp",
		Version: defaultVersion,
		Modules: []models.Module{{
			Name:    "keys",
			Package: "keys",
			Routes: []models.Route{{
				Method:      "POST",
				Path:        "/a",
				HandlerName: "create",
				Request:     req,
				Response:    resp,
			}},
		}},
	})
	require.NoError(t, err)
	require.NoError(t, specvalidate.Validate(context.Background(), []byte(spec)))
	var parsed struct {
		Components struct {
			Schemas map[string]*OpenAPISchema `yaml:"schemas"`
		} `yaml:"components"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &parsed))
	return parsed.Components.Schemas
}

// TestUntaggedFieldKeyedByGoName pins the issue's AReq and AResp: an
// untagged field is keyed by its Go name (Untagged, URL, ID) and a tagged
// one by its tag, the keys encoding/json writes, so a client reads what the
// server sends.
func TestUntaggedFieldKeyedByGoName(t *testing.T) {
	type aReq struct {
		Untagged bool
		URL      string
		Tagged   int `json:"tagged"`
	}
	type aResp struct{ ID int }
	req := &models.TypeInfo{Name: "AReq", Fields: []models.FieldInfo{
		bodyField("Untagged", "", prim(goTypeBool)),
		bodyField("URL", "", prim(goTypeString)),
		bodyField("Tagged", "tagged", prim(goTypeInt)),
	}}
	resp := &models.TypeInfo{Name: "AResp", Fields: []models.FieldInfo{bodyField("ID", "", prim(goTypeInt))}}
	schemas := postRouteSchemas(t, req, resp)
	require.Contains(t, schemas, "AReq")
	require.Contains(t, schemas, "AResp")
	assert.Equal(t, []string{"URL", "Untagged", "tagged"}, propertyKeys(schemas["AReq"]))
	assert.Equal(t, wireKeys(t, aReq{}), propertyKeys(schemas["AReq"]))
	assert.Equal(t, []string{"ID"}, propertyKeys(schemas["AResp"]))
	assert.Equal(t, wireKeys(t, aResp{}), propertyKeys(schemas["AResp"]))
}

// TestUntaggedFieldBesideCaseVariantTag pins the collision the lowered key
// caused: struct{ V int `validate:"required"`; X int `json:"v" validate:"required"` }
// is two keys on the wire, so it is two properties, both required, and the
// spec validates. Lowering V merged the two into one "v" property and listed
// "v" twice in required, which failed validation.
func TestUntaggedFieldBesideCaseVariantTag(t *testing.T) {
	type vReq struct {
		V int
		X int `json:"v"`
	}
	v := bodyField("V", "", prim(goTypeInt))
	v.Required = true
	x := bodyField("X", "v", prim(goTypeInt))
	x.Required = true
	schemas := postRouteSchemas(t, &models.TypeInfo{Name: "VReq", Fields: []models.FieldInfo{v, x}}, nil)
	require.Contains(t, schemas, "VReq")
	assert.Equal(t, []string{"V", "v"}, propertyKeys(schemas["VReq"]))
	assert.Equal(t, wireKeys(t, vReq{}), propertyKeys(schemas["VReq"]))
	assert.Equal(t, []string{"V", "v"}, schemas["VReq"].Required)
}

// TestUntaggedFieldKeyMatchesEmbedKey pins that one struct no longer mixes
// conventions: Price{ Cents; Amount int64 } keys the untagged non-struct
// embed by its type name (the JSON name embedProperty gives it) and the
// untagged named field by its Go name, Cents and Amount, as encoding/json
// writes them.
func TestUntaggedFieldKeyMatchesEmbedKey(t *testing.T) {
	type Cents int64
	type price struct {
		Cents
		Amount int64
	}
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	schema := gen.typeInfoToSchema(&models.TypeInfo{Name: "Price", Fields: []models.FieldInfo{
		bodyField(jsCents, jsCents, prim(goTypeInt64)),
		bodyField("Amount", "", prim(goTypeInt64)),
	}})
	require.NotNil(t, schema)
	assert.Equal(t, []string{"Amount", jsCents}, propertyKeys(schema))
	assert.Equal(t, wireKeys(t, price{}), propertyKeys(schema))
}

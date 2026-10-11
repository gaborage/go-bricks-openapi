package analyzer

import (
	"go/parser"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// multiPointerRequestTypes are request types written with two or more pointer
// levels. go-bricks binds a struct or one pointer to a struct and nothing
// else, so each of these is a non-struct request — even **Address, whose
// pointee is a struct.
var multiPointerRequestTypes = []string{"**Address", "***Address", "**string", "**[]Address", "*(*Address)"}

// TestMultiPointerRequestWarnsOnce pins #132: a request parameter written
// with two or more pointer levels is the request, not a framework parameter
// to skip, so it raises the one non-struct request warning per route with its
// type as written, in either parameter position. It resolves to no TypeInfo
// at all — the pointers are not peeled to resolve **Address as Address — so
// no requestBody is emitted and doctor counts the request untyped.
func TestMultiPointerRequestWarnsOnce(t *testing.T) {
	for _, typ := range multiPointerRequestTypes {
		for _, order := range []struct{ name, params string }{
			{"request_first", "req " + typ + ", ctx server.HandlerContext"},
			{"context_first", "ctx server.HandlerContext, req " + typ},
		} {
			t.Run(typ+"/"+order.name, func(t *testing.T) {
				route, warnings := analyzeNonStructRequest(t, order.params, "Address")
				assert.Equal(t, []string{wantNonStructRequestWarning(typ)}, warnings)
				assert.Nil(t, route.Request, "a multi-pointer request is never peeled to its pointee")
				require.NotNil(t, route.Response, "the response is resolved independently of the request")
				assert.Equal(t, "Address", route.Response.Name)
			})
		}
	}
}

// TestMultiPointerRequestWithUnresolvedResponseStillWarns pins that a
// multi-pointer request alone is enough to recognise the handler: with a
// response that resolves to nothing either, the warning still fires.
func TestMultiPointerRequestWithUnresolvedResponseStillWarns(t *testing.T) {
	route, warnings := analyzeNonStructRequest(t, "req **Address, ctx server.HandlerContext", "map[string]chan int")
	assert.Equal(t, []string{wantNonStructRequestWarning("**Address")}, warnings)
	assert.Nil(t, route.Request)
	assert.Nil(t, route.Response)
}

// TestIsMultiPointer pins the screen itself: two or more pointer levels
// match, whatever the pointee and however they are parenthesized (gofmt
// keeps *(*T)); one level or none does not.
func TestIsMultiPointer(t *testing.T) {
	cases := map[string]bool{
		"**Address":     true,
		"***Address":    true,
		"**pkg.Req":     true,
		"**[]Address":   true,
		"*Address":      false,
		"*[]Address":    false,
		"Address":       false,
		"[]*Address":    false,
		"*[]**Address":  false,
		"*(*Address)":   true,
		"(**Address)":   true,
		"*((*Address))": true,
		"(*Address)":    false,
		"((*Address))":  false,
		"*(Address)":    false,
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			expr, err := parser.ParseExpr(src)
			require.NoError(t, err)
			assert.Equal(t, want, isMultiPointer(expr))
		})
	}
}

package specvalidate

import (
	"context"
	"testing"
)

// validSpec is a minimal but complete OpenAPI 3.0 document. It exercises the
// success path without depending on the generator (so this package's tests stay
// independent of analyzer/generator changes).
const validSpec = `openapi: 3.0.1
info:
  title: Test API
  version: 1.0.0
paths: {}
`

// TestValidateAcceptsValidSpec locks the positive arm: a structurally valid
// OpenAPI 3.0 document returns nil.
func TestValidateAcceptsValidSpec(t *testing.T) {
	if err := Validate(context.Background(), []byte(validSpec)); err != nil {
		t.Fatalf("Validate rejected a valid spec: %v", err)
	}
}

// TestValidateRejectsMalformed locks the negative arms so this gate can never
// silently degrade into a no-op that passes everything.
func TestValidateRejectsMalformed(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			// Not parseable as YAML/JSON at all.
			name: "not_yaml",
			data: ":\n  not: [valid",
		},
		{
			// Parses as YAML but is not a valid OpenAPI document
			// (no openapi/info/paths).
			name: "missing_required_fields",
			data: "foo: bar\n",
		},
		{
			// Has an openapi version but no required info object.
			name: "missing_info",
			data: "openapi: 3.0.1\npaths: {}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(context.Background(), []byte(tt.data)); err == nil {
				t.Fatalf("Validate accepted invalid spec %q", tt.data)
			}
		})
	}
}

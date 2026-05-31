// Package specvalidate validates OpenAPI 3.0 documents in-process.
//
// It parses a spec from YAML or JSON bytes and validates it against the
// OpenAPI 3.0 specification using github.com/getkin/kin-openapi. This is the
// primary structural gate behind the `validate` subcommand and the generator's
// `--validate` flag: it is deterministic and needs no network or external
// toolchain (unlike the redocly step run in CI).
//
// This validation covers the *document* only — it confirms a generated or
// hand-edited OpenAPI file is structurally valid. Runtime request/response
// validation is out of scope.
package specvalidate

import (
	"context"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// Validate parses spec data (YAML or JSON) and validates it against OpenAPI 3.0
// in-process, honouring ctx for cancellation. It returns a readable error if the
// bytes cannot be parsed as a spec or if the parsed document is not valid
// OpenAPI 3.0.
func Validate(ctx context.Context, data []byte) error {
	loader := openapi3.NewLoader()
	loader.Context = ctx

	doc, err := loader.LoadFromData(data)
	if err != nil {
		return fmt.Errorf("parse spec: %w", err)
	}

	if err := doc.Validate(ctx); err != nil {
		return fmt.Errorf("invalid OpenAPI document: %w", err)
	}

	return nil
}

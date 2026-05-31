package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gaborage/go-bricks-openapi/internal/specvalidate"
)

// NewValidateCommand creates the validate command, which checks that an existing
// OpenAPI document (YAML or JSON) is structurally valid OpenAPI 3.0. It is the
// user-facing surface of the in-process validator that also backs the
// generator's --validate flag.
//
// This validates the *document* only. Runtime request/response validation is out
// of scope.
func NewValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [spec]",
		Short: "Validate an OpenAPI 3.0 specification document",
		Long: `Validates that an OpenAPI specification document is structurally valid
OpenAPI 3.0. The spec is parsed from YAML or JSON and checked in-process — no
network access or external toolchain is required.

The optional [spec] argument is the path to the document. When omitted it
defaults to the same file the generate command writes to (` + defaultOutputFile + `),
so a plain "generate" followed by "validate" operates on the same file.

This validates the document only; runtime request/response validation is out of
scope.`,
		Example: `  # Validate the default spec file (` + defaultOutputFile + `)
  go-bricks-openapi validate

  # Validate a specific file
  go-bricks-openapi validate docs/openapi.yaml`,
		Args: cobra.MaximumNArgs(1),
		// Errors are reported once by the root command's run() seam (a single
		// "Error:" line, no usage dump) rather than by cobra's default handler.
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath := defaultOutputFile
			if len(args) == 1 {
				specPath = args[0]
			}
			return runValidate(cmd, specPath)
		},
	}

	return cmd
}

func runValidate(cmd *cobra.Command, specPath string) error {
	// The path is user-supplied, but reading the file the user explicitly named
	// is the command's entire purpose (like `cat spec.yaml`), so there is no
	// traversal/privilege concern to guard against here.
	// #nosec G304 -- spec path is explicitly provided by the user to be validated
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("read spec %s: %w", specPath, err)
	}

	if err := specvalidate.Validate(cmd.Context(), data); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ %s is a valid OpenAPI 3.0 document\n", specPath)
	return nil
}

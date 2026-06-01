package commands

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// devVersion is the sentinel used when no real version is available — it matches
// the default value of main.version before ldflags or module metadata override it.
const devVersion = "dev"

// readBuildVersion returns the main module's version from the metadata the Go
// toolchain embeds in the binary, and whether a usable value was found. It is a
// package-level var so tests can substitute the lookup.
//
// `go install module@vX.Y.Z` records the tag here even though it never runs the
// Makefile/GoReleaser ldflags — this is what lets the go-install channel report
// an honest version. A local `go build` reports "(devel)" or empty, which is
// treated as "no usable version".
var readBuildVersion = func() (string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "", false
	}
	return v, true
}

// ResolveVersion picks the most authoritative version string available.
// Precedence: an ldflags-injected build version (GoReleaser / `make build`) wins;
// otherwise the main module version recorded by `go install`; otherwise the
// "dev" sentinel. injected is main.version, which defaults to "dev".
//
// Pseudo-versions (e.g. "v0.0.0-20230101000000-abcdef123456") from a
// commit-pinned `go install` are treated as usable and returned as-is — they
// are the correct version for that build.
func ResolveVersion(injected string) string {
	if injected != "" && injected != devVersion {
		return injected
	}
	if v, ok := readBuildVersion(); ok {
		return v
	}
	return devVersion
}

// NewVersionCommand creates the version command
func NewVersionCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display version information for go-bricks-openapi tool",
		Run: func(_ *cobra.Command, _ []string) {
			printVersion(version)
		},
	}

	return cmd
}

func printVersion(version string) {
	fmt.Printf("go-bricks-openapi version %s\n", version)
	fmt.Printf("Built with %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Println("OpenAPI specification version: 3.0.1")
}

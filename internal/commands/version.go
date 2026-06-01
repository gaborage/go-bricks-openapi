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

// usableBuildVersion extracts the main module's version from the build metadata
// the Go toolchain embeds in the binary. It normalizes the "no real version"
// markers — a missing BuildInfo (ok == false), an empty version, or the local
// "(devel)" placeholder — to ("", false); a release tag or a pseudo-version is
// returned as usable. Kept as a pure function (no I/O) so every branch is
// unit-testable with fabricated BuildInfo values.
func usableBuildVersion(info *debug.BuildInfo, ok bool) (string, bool) {
	if !ok || info == nil {
		return "", false
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "", false
	}
	return v, true
}

// readBuildVersion returns the main module's version recorded in the binary's
// build metadata, and whether a usable value was found. It is a package-level
// var so tests can substitute the lookup when exercising ResolveVersion.
//
// `go install module@vX.Y.Z` records the tag here even though it never runs the
// Makefile/GoReleaser ldflags — this is what lets the go-install channel report
// an honest version.
var readBuildVersion = func() (string, bool) {
	return usableBuildVersion(debug.ReadBuildInfo())
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

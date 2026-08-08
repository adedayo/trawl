// Package version is the single source of truth for the build identity of
// every Trawl binary.
//
// Trawl ships as three distinct executables — the Wails desktop application,
// the headless `trawl server`, and the worker entrypoints — built from one
// repository. Stamping `-X main.Version` would require a different ldflags
// string per binary, which is exactly the kind of value that drifts between a
// Dockerfile, a workflow and a release script until two artefacts from the
// same tag disagree about what they are. Stamping a shared package instead
// means one ldflags string covers all of them and disagreement is not
// expressible.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

// These are set at link time. See LDFlags below for the canonical string.
//
//	go build -ldflags "-X github.com/adedayo/trawl/pkg/version.Version=v1.2.3"
var (
	// Version is the release tag, e.g. "v1.2.3". "dev" in an unstamped build.
	Version = "dev"

	// Commit is the short git SHA the artefact was built from.
	Commit = ""

	// BuildDate is an RFC 3339 UTC timestamp of the build.
	BuildDate = ""
)

// Info is the resolved build identity, with build-info fallbacks applied.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"buildDate,omitempty"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

var (
	once     sync.Once
	resolved Info
)

// Get returns the build identity of the running binary.
//
// An unstamped binary is not necessarily an unknown one: `go install
// github.com/adedayo/trawl@v1.2.3` produces a binary with no ldflags but with
// a perfectly good module version and VCS stamp recorded by the toolchain.
// Reporting "dev" in that case would be throwing away information the runtime
// already holds, so the embedded build info is consulted before giving up.
func Get() Info {
	once.Do(func() {
		resolved = Info{
			Version:   Version,
			Commit:    Commit,
			BuildDate: BuildDate,
			GoVersion: runtime.Version(),
			Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		}

		bi, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}

		if resolved.Version == "dev" || resolved.Version == "" {
			if v := bi.Main.Version; v != "" && v != "(devel)" {
				resolved.Version = v
			}
		}

		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if resolved.Commit == "" {
					resolved.Commit = shortSHA(s.Value)
				}
			case "vcs.time":
				if resolved.BuildDate == "" {
					resolved.BuildDate = s.Value
				}
			case "vcs.modified":
				if s.Value == "true" && !strings.HasSuffix(resolved.Commit, "-dirty") {
					resolved.Commit += "-dirty"
				}
			}
		}
	})

	return resolved
}

// String renders the build identity for a `--version` flag or a log line.
func (i Info) String() string {
	s := i.Version
	if i.Commit != "" {
		s += " (" + i.Commit + ")"
	}
	if i.BuildDate != "" {
		s += " built " + i.BuildDate
	}
	return s
}

// Long renders the multi-line form used by the `version` subcommand.
func (i Info) Long(name string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", name, i.Version)
	if i.Commit != "" {
		fmt.Fprintf(&b, "  commit:     %s\n", i.Commit)
	}
	if i.BuildDate != "" {
		fmt.Fprintf(&b, "  built:      %s\n", i.BuildDate)
	}
	fmt.Fprintf(&b, "  go:         %s\n", i.GoVersion)
	fmt.Fprintf(&b, "  platform:   %s\n", i.Platform)
	return b.String()
}

// LDFlags returns the linker flags that stamp this package.
//
// It exists so that the release script, the Dockerfiles and the workflow can
// be checked against one definition rather than each carrying its own copy of
// a long import path that is wrong in a way nothing detects until a released
// binary reports "dev".
func LDFlags(version, commit, buildDate string) string {
	const pkg = "github.com/adedayo/trawl/pkg/version"
	return fmt.Sprintf(
		"-s -w -X %[1]s.Version=%[2]s -X %[1]s.Commit=%[3]s -X %[1]s.BuildDate=%[4]s",
		pkg, version, commit, buildDate,
	)
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

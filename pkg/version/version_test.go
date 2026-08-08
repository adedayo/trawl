package version

import (
	"strings"
	"testing"
)

// The point of the fallback is that an unstamped binary still reports
// something truthful. A test that only asserted "dev" would pass for a
// version package that had no fallback at all, so this asserts the weaker but
// meaningful property: never empty.
func TestGetNeverReturnsEmptyVersion(t *testing.T) {
	if got := Get().Version; got == "" {
		t.Fatal("Get().Version is empty; an unstamped build must still identify itself")
	}
}

func TestGetPopulatesRuntimeFields(t *testing.T) {
	info := Get()

	if info.GoVersion == "" {
		t.Error("GoVersion is empty")
	}
	if !strings.Contains(info.Platform, "/") {
		t.Errorf("Platform = %q, want GOOS/GOARCH", info.Platform)
	}
}

func TestGetIsStable(t *testing.T) {
	// Resolution is memoised, so two calls must not disagree. This guards the
	// sync.Once wiring rather than the values themselves.
	if Get() != Get() {
		t.Error("Get() returned differing values across calls")
	}
}

func TestInfoString(t *testing.T) {
	cases := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "version only",
			info: Info{Version: "v1.2.3"},
			want: "v1.2.3",
		},
		{
			name: "version and commit",
			info: Info{Version: "v1.2.3", Commit: "abc123"},
			want: "v1.2.3 (abc123)",
		},
		{
			name: "fully stamped",
			info: Info{Version: "v1.2.3", Commit: "abc123", BuildDate: "2026-01-01T00:00:00Z"},
			want: "v1.2.3 (abc123) built 2026-01-01T00:00:00Z",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.info.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInfoLongIncludesEveryPopulatedField(t *testing.T) {
	info := Info{
		Version:   "v1.2.3",
		Commit:    "abc123",
		BuildDate: "2026-01-01T00:00:00Z",
		GoVersion: "go1.26.1",
		Platform:  "darwin/arm64",
	}

	got := info.Long("trawl")

	for _, want := range []string{"trawl", "v1.2.3", "abc123", "2026-01-01T00:00:00Z", "go1.26.1", "darwin/arm64"} {
		if !strings.Contains(got, want) {
			t.Errorf("Long() = %q, missing %q", got, want)
		}
	}
}

func TestInfoLongOmitsEmptyFields(t *testing.T) {
	got := Info{Version: "dev", GoVersion: "go1.26.1", Platform: "linux/amd64"}.Long("trawl")

	if strings.Contains(got, "commit:") {
		t.Errorf("Long() emitted an empty commit line: %q", got)
	}
	if strings.Contains(got, "built:") {
		t.Errorf("Long() emitted an empty build date line: %q", got)
	}
}

// The whole reason LDFlags exists is that the import path is long, easy to
// mistype, and a typo produces a binary that reports "dev" with no build
// error. Asserting the exact path is therefore the assertion that matters.
func TestLDFlagsStampsTheCorrectImportPath(t *testing.T) {
	got := LDFlags("v1.2.3", "abc123", "2026-01-01T00:00:00Z")

	for _, want := range []string{
		"-X github.com/adedayo/trawl/pkg/version.Version=v1.2.3",
		"-X github.com/adedayo/trawl/pkg/version.Commit=abc123",
		"-X github.com/adedayo/trawl/pkg/version.BuildDate=2026-01-01T00:00:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("LDFlags() = %q, missing %q", got, want)
		}
	}

	if !strings.Contains(got, "-s -w") {
		t.Errorf("LDFlags() = %q, missing symbol-stripping flags", got)
	}
}

func TestShortSHA(t *testing.T) {
	if got := shortSHA("0123456789abcdef0123"); got != "0123456789ab" {
		t.Errorf("shortSHA() = %q, want %q", got, "0123456789ab")
	}
	if got := shortSHA("abc"); got != "abc" {
		t.Errorf("shortSHA() = %q, want %q", got, "abc")
	}
}

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adedayo/trawl/pkg/version"
)

// The CLI surface is a contract with the release script, the Dockerfiles, the
// worker entrypoints and anyone diagnosing a container. `server` is excluded
// deliberately: it blocks forever by design, and its coverage belongs in the
// server tests rather than here.
func TestRun(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  []string
		wantErr  []string
	}{
		{
			name:     "version",
			args:     []string{"version"},
			wantCode: 0,
			wantOut:  []string{"trawl", version.Get().Version},
		},
		{
			name:     "long version flag",
			args:     []string{"--version"},
			wantCode: 0,
			wantOut:  []string{"trawl", version.Get().Version},
		},
		{
			name:     "short version flag",
			args:     []string{"-v"},
			wantCode: 0,
			wantOut:  []string{"trawl", version.Get().Version},
		},
		{
			name:     "help",
			args:     []string{"help"},
			wantCode: 0,
			wantOut:  []string{"Usage:", "trawl server", "trawl version"},
		},
		{
			name:     "help flag",
			args:     []string{"--help"},
			wantCode: 0,
			wantOut:  []string{"Usage:"},
		},
		// Usage on a bare invocation goes to stderr with a non-zero code, so
		// that `trawl > out` in a script fails loudly instead of writing help
		// text into a file the script then treats as output.
		{
			name:     "no arguments",
			args:     nil,
			wantCode: 2,
			wantErr:  []string{"Usage:"},
		},
		{
			name:     "unknown command",
			args:     []string{"nonsense"},
			wantCode: 2,
			wantErr:  []string{"unknown command", "Usage:"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if got := run(tc.args, &stdout, &stderr); got != tc.wantCode {
				t.Errorf("run(%v) = %d, want %d", tc.args, got, tc.wantCode)
			}

			for _, want := range tc.wantOut {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout %q does not contain %q", stdout.String(), want)
				}
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr %q does not contain %q", stderr.String(), want)
				}
			}

			if len(tc.wantErr) == 0 && stderr.Len() != 0 {
				t.Errorf("unexpected stderr output: %q", stderr.String())
			}
		})
	}
}

// A version subcommand that prints "dev" from a released binary is worse than
// no version subcommand, because it is confidently wrong. The stamping itself
// is verified in pkg/version; this asserts the CLI actually surfaces it.
func TestVersionOutputReportsBuildIdentity(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run([]string{"version"}, &stdout, &stderr)

	got := stdout.String()
	for _, want := range []string{"go:", "platform:"} {
		if !strings.Contains(got, want) {
			t.Errorf("version output %q is missing %q", got, want)
		}
	}
}

package scanner

import (
	"context"
	"errors"
	"testing"
)

// The worker that previously enforced this has been retired, and its CI check
// with it. These cases exist so the control is asserted where it now lives: a
// safety property that is only documented is not enforced.
func TestScanRepoRejectsAuthenticatedURLs(t *testing.T) {
	rejected := []struct {
		name string
		url  string
	}{
		{"ssh scheme", "ssh://git@github.com/example/private.git"},
		{"scp-style git user", "git@github.com:example/private.git"},
		{"credentials before host", "https://user:pass@github.com/example/private.git"},
		{"token query parameter", "https://github.com/example/repo?token=abc123"},
		{"access_token query parameter", "https://github.com/example/repo?access_token=abc123"},
	}

	// A nil store and bus are safe here precisely because the guard returns
	// before either is touched. If that ordering regresses, this panics rather
	// than passing quietly.
	s := NewSecretScanner(nil, nil)

	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			err := s.ScanRepo(context.Background(), tc.url)
			if err == nil {
				t.Fatalf("ScanRepo(%q) = nil, want rejection", tc.url)
			}
			if !errors.Is(err, ErrAuthenticatedRepo) {
				t.Fatalf("ScanRepo(%q) error = %v, want ErrAuthenticatedRepo", tc.url, err)
			}
		})
	}
}

func TestScanRepoAcceptsPublicURLs(t *testing.T) {
	// These must not match the guard. A pattern that rejected ordinary public
	// URLs would fail closed and disable repository scanning altogether, which
	// is a silent loss of capability rather than a visible error.
	accepted := []string{
		"https://github.com/example/repo",
		"https://github.com/example/repo.git",
		"https://gitlab.com/group/subgroup/project.git",
	}

	for _, url := range accepted {
		if authenticatedRepoURL.MatchString(url) {
			t.Errorf("public URL %q was classified as requiring authentication", url)
		}
	}
}

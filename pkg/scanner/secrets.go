package scanner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/adedayo/checkmate/pkg/sdk"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/google/uuid"
)

// authenticatedRepoURL matches repository URLs that carry, or imply, credentials.
//
// Trawl scans repositories that are public. Accepting a URL with credentials in
// it would mean cloning something the operator may have authority over but this
// tool has not been authorised to read, and it would write those credentials
// into process arguments and any error we log. Refusing is not a limitation to
// be worked around later: the scope authorisation document states this control,
// so it has to be enforced where the clone happens.
//
// The `//[^/@]*@` alternative covers URL userinfo — `https://user:pass@host/…`.
// The shell worker this replaces matched `\.git.*@`, which required a literal
// `.git` before the `@` and so let the commonest credential-bearing form
// through entirely. That gap is why this is now tested rather than trusted.
var authenticatedRepoURL = regexp.MustCompile(`(ssh://|git@|//[^/@]*@|\.git.*@|token=|access_token=)`)

// ErrAuthenticatedRepo is returned when a repository URL requires authentication.
var ErrAuthenticatedRepo = fmt.Errorf("repo URL appears to require authentication, which is not supported")

type SecretScanner struct {
	store    store.Store
	eventBus event.Bus
}

func NewSecretScanner(s store.Store, eb event.Bus) *SecretScanner {
	return &SecretScanner{
		store:    s,
		eventBus: eb,
	}
}

// ScanRepo runs Checkmate natively against a given repository URL asynchronously.
func (s *SecretScanner) ScanRepo(ctx context.Context, repoURL string) error {
	// Refuse credential-bearing URLs before anything is recorded or cloned. The
	// URL is not echoed back: it is the thing that may contain the secret.
	if authenticatedRepoURL.MatchString(repoURL) {
		return ErrAuthenticatedRepo
	}

	// 1. Ensure Repository Asset exists
	assetID := uuid.New().String()
	repoAsset := store.Asset{
		ID:              assetID,
		Type:            "repository",
		Value:           repoURL,
		Status:          "active",
		DiscoverySource: "checkmate",
		FirstSeen:       time.Now(),
		LastSeen:        time.Now(),
	}
	_ = s.store.SaveAsset(ctx, &repoAsset)

	if s.eventBus != nil {
		s.eventBus.Publish(ctx, event.Event{
			Type:    event.EventAssetUpdated,
			Payload: repoAsset,
		})
	}

	// 2. Clone repository to temp directory
	tempDir, err := os.MkdirTemp("", "trawl-repo-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir) // clean up

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, tempDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository %s: %w", repoURL, err)
	}

	// 3. Initialize CheckMate SDK scanner
	opts := sdk.Options{
		SensitiveFilesOnly: false,
		CalculateChecksum:  true,
		ExcludeTestFiles:   true,
	}
	scanner := sdk.NewScanner(opts)

	// 4. Scan the stream and ingest natively
	findingsChan := scanner.ScanStream(ctx, tempDir)

	for f := range findingsChan {
		// Clean up file path relative to repo root
		cleanPath := strings.TrimPrefix(f.File, tempDir+"/")

		sf := store.SecretFinding{
			ID:          uuid.New().String(),
			AssetID:     repoAsset.ID,
			RepoURL:     repoURL,
			RuleID:      f.RuleID,
			SecretType:  string(f.SecretType),
			FilePath:    cleanPath,
			StartLine:   f.Line,
			RedactedRef: f.SecretChecksum,
			FirstSeen:   time.Now(),
		}

		if err := s.store.SaveSecretFinding(ctx, &sf); err != nil {
			fmt.Printf("failed to save secret finding: %v\n", err)
			continue
		}

		// Emit event to stream to UI
		if s.eventBus != nil {
			s.eventBus.Publish(ctx, event.Event{
				Type:    event.EventFindingNew,
				Payload: sf,
			})
		}
	}

	return nil
}

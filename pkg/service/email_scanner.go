package service

import (
	"context"
	"fmt"

	vpkg "github.com/adedayo/vantage/pkg"
	vaudit "github.com/adedayo/vantage/pkg/audit"

	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/store"
)

// EmailScannerService assesses a domain's email-authentication posture.
//
// It runs through the vantage adapter rather than issuing its own DNS
// queries. The implementation it replaced called net.LookupTXT and built its
// own resolver clients, so it consulted neither the authorised scope nor the
// deployment's egress policy: a domain nobody had authorised was queried, and
// an operator who had restricted the engine to a nominated resolver found this
// capability quietly reaching a different one. Routing through the adapter
// closes that, because the guard then lives in the transport rather than in a
// convention every call site has to remember.
type EmailScannerService struct {
	store store.Store
}

func NewEmailScannerService(s store.Store) *EmailScannerService {
	return &EmailScannerService{store: s}
}

// ScanAndSave assesses the domain and persists the posture.
//
// The authorised scope is a required argument rather than an optional one. A
// default of "no scope" would fail closed and merely confuse; a default of
// "any scope" would reinstate the bypass this service exists to remove.
func (svc *EmailScannerService) ScanAndSave(
	ctx context.Context,
	domain string,
	authorisedDomains []string,
	consentedEndpoints []string,
) (store.EmailPosture, error) {
	domain = NormaliseDomain(domain)
	if domain == "" {
		return store.EmailPosture{}, fmt.Errorf("email scan: a domain is required")
	}

	scope := vadapter.NewScope(authorisedDomains, consentedEndpoints)
	if !scope.PermitsTarget(domain) {
		return store.EmailPosture{}, fmt.Errorf(
			"email scan: %q is outside the authorised scope", domain)
	}

	resolver := vpkg.NewClient(vpkg.Config{})
	assessor, _, _, err := vadapter.NewScopedAssessor(resolver, scope)
	if err != nil {
		return store.EmailPosture{}, fmt.Errorf("email scan: building the assessor: %w", err)
	}

	adapter, err := vadapter.New(assessor)
	if err != nil {
		return store.EmailPosture{}, fmt.Errorf("email scan: building the adapter: %w", err)
	}

	res, assessErr := adapter.Assess(ctx, vadapter.Request{
		Domain:  domain,
		Profile: string(vaudit.ProfileStandard),
		Scope:   scope,
	})

	// A failed assessment still yields a posture, and the posture is still
	// written. Its controls carry check_failed with the reason, which is the
	// record an operator needs; discarding it would leave the previous run's
	// result standing as though it were current — a stale pass presenting as
	// a fresh one.
	if res.EmailPosture == nil {
		if assessErr != nil {
			return store.EmailPosture{}, fmt.Errorf("email scan failed: %w", assessErr)
		}
		return store.EmailPosture{}, fmt.Errorf(
			"email scan: no email-authentication check ran for %s", domain)
	}

	posture := *res.EmailPosture
	if err := svc.store.SaveEmailPosture(ctx, &posture); err != nil {
		return store.EmailPosture{}, fmt.Errorf("failed to save email posture: %w", err)
	}
	return posture, assessErr
}

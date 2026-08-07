package core

import (
	"context"
	"fmt"

	"github.com/adedayo/trawl/pkg/service"
)

// DiscoverRelatedDomains looks for domains that share certificates with the
// authorised ones and records them as proposals awaiting a decision.
//
// It assesses nothing. The only egress is to the certificate transparency
// services, which the operator has consented to separately, so a domain
// surfaced here has had no packet sent to it. That separation is the point:
// the operator decides what is theirs before anything touches it.
func (c *Core) DiscoverRelatedDomains(
	ctx context.Context,
	opts service.DiscoveryOptions,
) (service.DiscoveryResult, error) {
	scope := c.Scope(ctx)
	if !scope.Authorised() {
		return service.DiscoveryResult{}, fmt.Errorf(
			"no signed authorisation is on record; discovery refused")
	}

	// Domains already decided upon are excluded at source, so the proposal
	// list is a queue of open questions rather than a list of everything ever
	// seen.
	known := map[string]bool{}
	for _, d := range scope.SeedDomainsList {
		known[service.NormaliseDomain(d)] = true
	}
	for _, d := range scope.DismissedDomains {
		known[service.NormaliseDomain(d)] = true
	}

	result, err := c.assessment.DiscoverRelated(
		ctx, scope.SeedDomainsList, scope.ConsentedEndpoints, known, opts)
	if err != nil {
		return service.DiscoveryResult{}, err
	}

	// The proposals replace rather than accumulate. A stale proposal whose
	// certificate has since lapsed should disappear from the queue, and the
	// operator's own decisions are held in the scope and dismissal lists, so
	// nothing they have said is lost by rebuilding this.
	scope.ProposedDomains = result.Proposals
	if err := c.SaveScope(ctx, scope); err != nil {
		return service.DiscoveryResult{}, err
	}

	return result, nil
}

// AuthoriseProposedDomains moves proposals into the authorised scope.
//
// This is the moment authority is asserted, so it is deliberately explicit and
// deliberately per-domain: the operator is stating that each of these belongs
// to their organisation and that they may assess it.
func (c *Core) AuthoriseProposedDomains(ctx context.Context, domains []string) error {
	return c.decideProposals(ctx, domains, true)
}

// DismissProposedDomains rules proposals out.
//
// The domain is remembered as dismissed rather than merely removed, so that
// the next discovery pass does not raise it again.
func (c *Core) DismissProposedDomains(ctx context.Context, domains []string) error {
	return c.decideProposals(ctx, domains, false)
}

// decideProposals applies one decision to a set of proposed domains.
func (c *Core) decideProposals(ctx context.Context, domains []string, authorise bool) error {
	scope := c.Scope(ctx)
	if !scope.Authorised() {
		return fmt.Errorf("no signed authorisation is on record; the scope cannot be changed")
	}

	decided := map[string]bool{}
	for _, d := range domains {
		if n := service.NormaliseDomain(d); n != "" {
			decided[n] = true
		}
	}
	if len(decided) == 0 {
		return nil
	}

	// Only domains actually on the proposal list may be decided here. A
	// transport that could authorise an arbitrary domain through this path
	// would be a transport that could widen the scope without the operator
	// ever seeing the evidence for it.
	proposed := map[string]bool{}
	for _, p := range scope.ProposedDomains {
		proposed[p.Domain] = true
	}
	for d := range decided {
		if !proposed[d] {
			return fmt.Errorf("%q is not a proposed domain; only discovered domains may be decided here", d)
		}
	}

	remaining := make([]service.ProposedDomain, 0, len(scope.ProposedDomains))
	for _, p := range scope.ProposedDomains {
		if !decided[p.Domain] {
			remaining = append(remaining, p)
		}
	}
	scope.ProposedDomains = remaining

	for d := range decided {
		if authorise {
			if !containsDomain(scope.SeedDomainsList, d) {
				scope.SeedDomainsList = append(scope.SeedDomainsList, d)
			}
			continue
		}
		if !containsDomain(scope.DismissedDomains, d) {
			scope.DismissedDomains = append(scope.DismissedDomains, d)
		}
	}

	return c.SaveScope(ctx, scope)
}

// RestoreDismissedDomain returns a dismissed domain to consideration, so that a
// decision made in error is reversible without re-running discovery.
func (c *Core) RestoreDismissedDomain(ctx context.Context, domain string) error {
	scope := c.Scope(ctx)
	if !scope.Authorised() {
		return fmt.Errorf("no signed authorisation is on record; the scope cannot be changed")
	}

	target := service.NormaliseDomain(domain)
	remaining := make([]string, 0, len(scope.DismissedDomains))
	for _, d := range scope.DismissedDomains {
		if service.NormaliseDomain(d) != target {
			remaining = append(remaining, d)
		}
	}
	scope.DismissedDomains = remaining
	return c.SaveScope(ctx, scope)
}

// ProposedDomains returns the current review queue.
func (c *Core) ProposedDomains(ctx context.Context) []service.ProposedDomain {
	return c.Scope(ctx).ProposedDomains
}

// DismissedDomains returns the domains the operator has ruled out.
func (c *Core) DismissedDomains(ctx context.Context) []string {
	return c.Scope(ctx).DismissedDomains
}

func containsDomain(list []string, domain string) bool {
	for _, d := range list {
		if service.NormaliseDomain(d) == domain {
			return true
		}
	}
	return false
}

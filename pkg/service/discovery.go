package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	vct "github.com/adedayo/vantage/pkg/ct"

	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
)

// ProposedDomain is a registrable domain that certificate evidence suggests
// belongs to the operator, awaiting their decision.
//
// It is deliberately not an asset. Nothing about it has been queried and
// nothing will be until somebody authorises it: the whole point of the
// discover-then-authorise split is that certificate co-tenancy is evidence of
// ownership, not proof of it, and only the operator can close that gap.
type ProposedDomain struct {
	// Domain is the registrable domain discovered.
	Domain string `json:"domain"`
	// Via is the authorised domain whose certificate named this one. It is the
	// evidence for the proposal, and the reason a reviewer can make a decision
	// rather than a guess.
	Via string `json:"via"`
	// Depth is how many hops of co-tenancy from an authorised domain this was.
	// One means it shared a certificate with a domain already in scope; higher
	// values are inferences built on inferences and warrant more scepticism.
	Depth int `json:"depth"`
	// Hostnames are the names the logs hold under this domain, as a measure of
	// how much surface authorising it would add.
	Hostnames []string `json:"hostnames"`
	// Issuer is a certificate authority seen issuing for the domain, which
	// often corroborates a shared procurement route.
	Issuer string `json:"issuer"`
	// DiscoveredAt is when the proposal was raised.
	DiscoveredAt string `json:"discoveredAt"`
}

// DiscoveryResult is one discovery pass.
type DiscoveryResult struct {
	// Proposals are the domains awaiting a decision, excluding anything
	// already authorised or previously dismissed.
	Proposals []ProposedDomain `json:"proposals"`
	// Searched are the authorised domains that were expanded.
	Searched []string `json:"searched"`
	// BudgetExhausted reports that the walk stopped early, so the proposal
	// list may be incomplete.
	BudgetExhausted bool `json:"budgetExhausted"`
	// Errors are per-domain failures, so a partial discovery says what it
	// missed rather than presenting itself as complete.
	Errors []string `json:"errors"`
}

// DiscoveryOptions tune how far certificate evidence is followed.
//
// Zero values mean the vantage defaults, which are the conservative ones.
type DiscoveryOptions struct {
	// Depth is how many hops of co-tenancy to follow.
	Depth int `json:"depth"`
	// Budget caps how many domains are enumerated in total.
	Budget int `json:"budget"`
	// MaxSANs is the largest certificate from which shared ownership is
	// inferred.
	MaxSANs int `json:"maxSans"`
}

// DiscoverRelated finds domains that share certificates with the authorised
// ones, without assessing anything.
//
// This contacts the certificate transparency services and nothing else. That
// is what makes it safe to run against domains not yet in scope: the logs are
// public, the query discloses only the authorised domain, and no packet is sent
// to any discovered domain's own infrastructure. Assessment stays behind the
// scope guard where it belongs.
//
// consentedEndpoints still gates the log services themselves, because consent
// to assess a domain is not consent to disclose it to a third party.
func (svc *AssessmentService) DiscoverRelated(
	ctx context.Context,
	authorisedDomains []string,
	consentedEndpoints []string,
	known map[string]bool,
	opts DiscoveryOptions,
) (DiscoveryResult, error) {
	if len(authorisedDomains) == 0 {
		return DiscoveryResult{}, fmt.Errorf("discovery: no authorised domains to expand from")
	}

	// The HTTP client is the scoped one, so a log service the operator has not
	// consented to is refused at the transport rather than merely unused.
	scope := vadapter.NewScope(authorisedDomains, consentedEndpoints)
	guardedHTTP := vadapter.NewScopedHTTPClient(nil, scope)
	source := vct.DefaultSourcesWith(guardedHTTP)

	pivot := vct.PivotOptions{
		Depth:              opts.Depth,
		Budget:             opts.Budget,
		MaxSANsForRelation: opts.MaxSANs,
	}

	result := DiscoveryResult{}
	// Proposals are keyed by domain so that a sibling reachable from two
	// authorised domains is offered once, with the shortest path retained.
	byDomain := map[string]ProposedDomain{}
	now := time.Now().UTC().Format(time.RFC3339)

	for _, seed := range authorisedDomains {
		seed = NormaliseDomain(seed)
		if seed == "" {
			continue
		}
		result.Searched = append(result.Searched, seed)

		exp, err := vct.Expand(ctx, source, seed, pivot)
		if err != nil {
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s: %v", seed, err))
			continue
		}
		if exp.BudgetExhausted {
			result.BudgetExhausted = true
		}
		for domain, expErr := range exp.Errors {
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s (found via %s): %v", domain, seed, expErr))
		}

		for _, d := range exp.Discoveries {
			if d.Depth == 0 {
				continue
			}
			// Anything the operator has already ruled on — authorised or
			// dismissed — must not be offered again. Re-proposing a domain
			// somebody has already excluded turns a review queue into noise.
			if known[d.Domain] || scope.PermitsTarget(d.Domain) {
				continue
			}

			candidate := ProposedDomain{
				Domain:       d.Domain,
				Via:          d.Via,
				Depth:        d.Depth,
				Hostnames:    d.Result.Hosts,
				Issuer:       firstIssuer(d.Result),
				DiscoveredAt: now,
			}
			if existing, ok := byDomain[d.Domain]; ok && existing.Depth <= candidate.Depth {
				continue
			}
			byDomain[d.Domain] = candidate
		}
	}

	for _, p := range byDomain {
		result.Proposals = append(result.Proposals, p)
	}
	// Shallowest first, then alphabetically: the most defensible proposals lead
	// the queue, and the order is stable between runs so a reviewer returning
	// to the list finds it where they left it.
	sort.Slice(result.Proposals, func(i, j int) bool {
		if result.Proposals[i].Depth != result.Proposals[j].Depth {
			return result.Proposals[i].Depth < result.Proposals[j].Depth
		}
		return result.Proposals[i].Domain < result.Proposals[j].Domain
	})
	sort.Strings(result.Errors)

	return result, nil
}

// firstIssuer returns a certificate authority seen for the domain, if any.
func firstIssuer(r vct.Result) string {
	for _, c := range r.Certificates {
		if s := strings.TrimSpace(c.Issuer); s != "" {
			return s
		}
	}
	return ""
}

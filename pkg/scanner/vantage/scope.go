package vantage

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/miekg/dns"

	vantage "github.com/adedayo/vantage/pkg"
)

// Scope is the set of names an assessment is authorised to contact.
//
// It is enforced at the transport rather than at the point a check decides
// what to look up. Those are not the same guarantee: DNS assessment follows
// references it discovers — a CNAME to a third party, an MX at another
// operator, a nameserver in someone else's zone — so the set of names a run
// would query is not knowable in advance. Filtering requests before they are
// made can only refuse what was anticipated; refusing at the transport is what
// makes "an out-of-scope target emits no packet" an assertion rather than a
// hope.
type Scope struct {
	// domains are the authorised apexes. A name is in scope if it equals one
	// of these or is a subdomain of it.
	domains []string
	// endpoints are third-party service hosts the operator has consented to,
	// held separately from target scope. Consent to assess a domain is not
	// consent to disclose it to a certificate transparency log, so the two
	// lists are never merged.
	endpoints map[string]bool
}

// NewScope builds a scope from authorised domains and consented third-party
// endpoints.
func NewScope(domains, endpoints []string) *Scope {
	s := &Scope{endpoints: map[string]bool{}}
	for _, d := range domains {
		if n := normalise(d); n != "" {
			s.domains = append(s.domains, n)
		}
	}
	for _, e := range endpoints {
		if n := normalise(e); n != "" {
			s.endpoints[n] = true
		}
	}
	return s
}

// PermitsTarget reports whether name is within the authorised domains.
func (s *Scope) PermitsTarget(name string) bool {
	n := normalise(name)
	if n == "" || s == nil {
		return false
	}
	for _, d := range s.domains {
		if n == d || strings.HasSuffix(n, "."+d) {
			return true
		}
	}
	return s.permitsReportAuthorisation(n)
}

// reportAuthorisationInfix is the fixed label pair that RFC 7489 §7.1 places
// between an authorised domain and the third party reporting on its behalf.
const reportAuthorisationInfix = "._report._dmarc."

// permitsReportAuthorisation admits the DMARC external-reporting
// authorisation record, which is the one name an assessment must read that
// lies, by construction, in somebody else's zone.
//
// The record at "<target>._report._dmarc.<destination>" is published by the
// destination to declare that it accepts reports for the target. Refusing it
// does not withhold anything: the target's own DMARC record already names
// every destination publicly, and this query adds only the target's name — the
// one name the operator has authorised — to a lookup that reveals no other
// asset in the portfolio.
//
// Refusing it does, however, break the rule that reads it. Vantage can no
// longer distinguish "the destination never authorised this" from "we were not
// allowed to ask", and a portfolio would carry a permanent, unfixable advisory
// against correctly configured mail. A guard that makes a check unusable while
// protecting nothing is not caution; it is a false sense of it.
//
// The prefix must be an authorised domain, not merely contain one. Admitting
// any name with the infix would let "attacker.example._report._dmarc.evil" out
// through a guard that exists to stop exactly that.
func (s *Scope) permitsReportAuthorisation(n string) bool {
	for _, d := range s.domains {
		if strings.HasPrefix(n, d+reportAuthorisationInfix) {
			return true
		}
	}
	return false
}

// PermitsEndpoint reports whether host is a consented third-party service.
func (s *Scope) PermitsEndpoint(host string) bool {
	if s == nil {
		return false
	}
	return s.endpoints[normalise(host)]
}

// normalise lower-cases a name and strips the trailing dot and any port, so
// that scope comparisons cannot be defeated by presentation.
func normalise(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, ".")
	if h, _, err := net.SplitHostPort(n); err == nil {
		n = h
	}
	return n
}

// Denial records a refused request, so that a refusal is auditable rather than
// merely silent.
type Denial struct {
	// Kind is "dns" or "http".
	Kind string
	// Name is the host or query name that was refused.
	Name string
}

// scopeGuard is the shared refusal bookkeeping for both transports.
type scopeGuard struct {
	scope *Scope

	mu      sync.Mutex
	denials []Denial
}

func (g *scopeGuard) deny(kind, name string) error {
	g.mu.Lock()
	g.denials = append(g.denials, Denial{Kind: kind, Name: name})
	g.mu.Unlock()
	return fmt.Errorf("trawl: %s request for %q is outside the authorised scope: %w",
		kind, name, vantage.ErrOutOfScope)
}

// Denials returns the refused requests recorded so far.
func (g *scopeGuard) Denials() []Denial {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]Denial(nil), g.denials...)
}

// ScopedResolver wraps a resolver and refuses queries for names outside scope.
type ScopedResolver struct {
	scopeGuard
	inner vantage.Resolver
}

// NewScopedResolver wraps inner so that only in-scope names may be queried.
func NewScopedResolver(inner vantage.Resolver, scope *Scope) *ScopedResolver {
	return &ScopedResolver{scopeGuard: scopeGuard{scope: scope}, inner: inner}
}

var _ vantage.Resolver = (*ScopedResolver)(nil)

func (r *ScopedResolver) permit(name string) error {
	if r.scope.PermitsTarget(name) {
		return nil
	}
	return r.deny("dns", name)
}

// ExchangeFrom implements vantage.Resolver.
func (r *ScopedResolver) ExchangeFrom(ctx context.Context, name string, qtype uint16) (*dns.Msg, string, error) {
	if err := r.permit(name); err != nil {
		return nil, "", err
	}
	return r.inner.ExchangeFrom(ctx, name, qtype)
}

// ExchangeRawFrom implements vantage.Resolver.
func (r *ScopedResolver) ExchangeRawFrom(ctx context.Context, name string, qtype uint16) (*dns.Msg, string, error) {
	if err := r.permit(name); err != nil {
		return nil, "", err
	}
	return r.inner.ExchangeRawFrom(ctx, name, qtype)
}

// ExchangeDNSSECRawFrom implements vantage.Resolver.
func (r *ScopedResolver) ExchangeDNSSECRawFrom(ctx context.Context, name string, qtype uint16) (*dns.Msg, string, error) {
	if err := r.permit(name); err != nil {
		return nil, "", err
	}
	return r.inner.ExchangeDNSSECRawFrom(ctx, name, qtype)
}

// ExchangeWithServer implements vantage.Resolver.
//
// The query name is checked, not the server addressed. A check that asks a
// specific nameserver a question about an in-scope zone is legitimate — that
// is how delegation and zone-transfer assessment work — whereas asking any
// server about a name outside scope is not.
func (r *ScopedResolver) ExchangeWithServer(ctx context.Context, server, name string, qtype uint16) (*dns.Msg, error) {
	if err := r.permit(name); err != nil {
		return nil, err
	}
	return r.inner.ExchangeWithServer(ctx, server, name, qtype)
}

// ExchangeDNSSECWithServer implements vantage.Resolver.
func (r *ScopedResolver) ExchangeDNSSECWithServer(ctx context.Context, server, name string, qtype uint16) (*dns.Msg, error) {
	if err := r.permit(name); err != nil {
		return nil, err
	}
	return r.inner.ExchangeDNSSECWithServer(ctx, server, name, qtype)
}

// Servers implements vantage.Resolver.
func (r *ScopedResolver) Servers() []string { return r.inner.Servers() }

// ScopedHTTPClient wraps an HTTP client and refuses requests to hosts that are
// neither in-scope targets nor consented third-party endpoints.
type ScopedHTTPClient struct {
	scopeGuard
	inner vantage.Doer
}

// NewScopedHTTPClient wraps inner with scope enforcement. A nil inner uses the
// vantage default client.
func NewScopedHTTPClient(inner vantage.Doer, scope *Scope) *ScopedHTTPClient {
	return &ScopedHTTPClient{
		scopeGuard: scopeGuard{scope: scope},
		inner:      vantage.HTTPOr(inner, vantage.HTTPOptions{}),
	}
}

var _ vantage.Doer = (*ScopedHTTPClient)(nil)

// Do implements vantage.Doer.
func (c *ScopedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()

	// A target host is permitted if it is in scope. MTA-STS policy hosts and
	// takeover corroboration both address names under the assessed domain, so
	// this is the ordinary path.
	if c.scope.PermitsTarget(host) {
		return c.inner.Do(req)
	}
	// Otherwise it must be an endpoint the operator explicitly consented to.
	// An unlisted third party is refused even though the check that wants it
	// was selected: consent to run a check is not consent to disclose the
	// portfolio to whoever that check happens to query.
	if c.scope.PermitsEndpoint(host) {
		return c.inner.Do(req)
	}
	return nil, c.deny("http", host)
}

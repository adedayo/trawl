package vantage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/miekg/dns"

	vantage "github.com/adedayo/vantage/pkg"
)

// countingResolver is an instrumented transport. It records every query that
// reaches it, so a test can assert on packets that would have left the host
// rather than on the tool's stated intentions.
type countingResolver struct {
	mu      sync.Mutex
	queried []string
}

func (c *countingResolver) record(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queried = append(c.queried, name)
}

func (c *countingResolver) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.queried)
}

func (c *countingResolver) ExchangeFrom(_ context.Context, name string, _ uint16) (*dns.Msg, string, error) {
	c.record(name)
	return new(dns.Msg), "10.0.0.1:53", nil
}

func (c *countingResolver) ExchangeRawFrom(_ context.Context, name string, _ uint16) (*dns.Msg, string, error) {
	c.record(name)
	return new(dns.Msg), "10.0.0.1:53", nil
}

func (c *countingResolver) ExchangeDNSSECRawFrom(_ context.Context, name string, _ uint16) (*dns.Msg, string, error) {
	c.record(name)
	return new(dns.Msg), "10.0.0.1:53", nil
}

func (c *countingResolver) ExchangeWithServer(_ context.Context, _, name string, _ uint16) (*dns.Msg, error) {
	c.record(name)
	return new(dns.Msg), nil
}

func (c *countingResolver) ExchangeDNSSECWithServer(_ context.Context, _, name string, _ uint16) (*dns.Msg, error) {
	c.record(name)
	return new(dns.Msg), nil
}

func (c *countingResolver) Servers() []string { return []string{"10.0.0.1:53"} }

var _ vantage.Resolver = (*countingResolver)(nil)

// TestOutOfScopeEmitsZeroQueries is the required CI check for Phase 3.
//
// It asserts the property directly: however the assessment logic is asked to
// behave, no query for an unauthorised name reaches the transport. Counting at
// the transport is the point — a test that only checked the returned error
// would still pass if the packet went out and the result was discarded.
func TestOutOfScopeEmitsZeroQueries(t *testing.T) {
	inner := &countingResolver{}
	scope := NewScope([]string{"authorised.example"}, nil)
	r := NewScopedResolver(inner, scope)

	ctx := context.Background()
	unauthorised := []string{
		"forbidden.example",
		"sub.forbidden.example",
		// A name that merely ends in the authorised string but is a different
		// registration. Suffix matching without the dot boundary would admit
		// this, which is how scope checks are usually got wrong.
		"notauthorised.example",
		"authorised.example.attacker.test",
	}

	for _, name := range unauthorised {
		if _, _, err := r.ExchangeFrom(ctx, name, dns.TypeA); !errors.Is(err, vantage.ErrOutOfScope) {
			t.Errorf("ExchangeFrom(%q) error = %v, want ErrOutOfScope", name, err)
		}
		if _, _, err := r.ExchangeRawFrom(ctx, name, dns.TypeA); !errors.Is(err, vantage.ErrOutOfScope) {
			t.Errorf("ExchangeRawFrom(%q) error = %v, want ErrOutOfScope", name, err)
		}
		if _, _, err := r.ExchangeDNSSECRawFrom(ctx, name, dns.TypeA); !errors.Is(err, vantage.ErrOutOfScope) {
			t.Errorf("ExchangeDNSSECRawFrom(%q) error = %v, want ErrOutOfScope", name, err)
		}
		if _, err := r.ExchangeWithServer(ctx, "10.0.0.9:53", name, dns.TypeA); !errors.Is(err, vantage.ErrOutOfScope) {
			t.Errorf("ExchangeWithServer(%q) error = %v, want ErrOutOfScope", name, err)
		}
		if _, err := r.ExchangeDNSSECWithServer(ctx, "10.0.0.9:53", name, dns.TypeA); !errors.Is(err, vantage.ErrOutOfScope) {
			t.Errorf("ExchangeDNSSECWithServer(%q) error = %v, want ErrOutOfScope", name, err)
		}
	}

	if got := inner.count(); got != 0 {
		t.Fatalf("%d queries reached the transport for out-of-scope names: %v", got, inner.queried)
	}
	if len(r.Denials()) == 0 {
		t.Error("refusals must be recorded, so that a coverage gap can name its cause")
	}
}

// TestInScopeQueriesReachTheTransport is the other half: the guard must not be
// so strict that it refuses the assessment it was built to permit. A guard that
// blocks everything trivially passes the zero-egress test.
func TestInScopeQueriesReachTheTransport(t *testing.T) {
	inner := &countingResolver{}
	r := NewScopedResolver(inner, NewScope([]string{"authorised.example"}, nil))

	for _, name := range []string{
		"authorised.example",
		"www.authorised.example",
		"deep.nested.authorised.example",
		"AUTHORISED.EXAMPLE",  // case must not defeat the check
		"authorised.example.", // nor a trailing dot
	} {
		if _, _, err := r.ExchangeFrom(context.Background(), name, dns.TypeA); err != nil {
			t.Errorf("ExchangeFrom(%q) unexpectedly refused: %v", name, err)
		}
	}
	if got := inner.count(); got != 5 {
		t.Fatalf("queries reaching the transport = %d, want 5", got)
	}
}

// TestReportAuthorisationRecordIsInScope covers the one name an assessment
// must read that lies, by construction, in a third party's zone.
//
// RFC 7489 §7.1 places the authorisation for an external reporting destination
// at "<target>._report._dmarc.<destination>". Refusing it withholds nothing —
// the target's own DMARC record already names every destination publicly — but
// it does make the rule that reads it permanently unusable, since the check
// cannot then tell "never authorised" from "not allowed to ask".
func TestReportAuthorisationRecordIsInScope(t *testing.T) {
	inner := &countingResolver{}
	r := NewScopedResolver(inner, NewScope([]string{"authorised.example"}, nil))

	for _, name := range []string{
		"authorised.example._report._dmarc.thirdparty.test",
		"authorised.example._report._dmarc.inbox.reporting.test",
		"AUTHORISED.EXAMPLE._REPORT._DMARC.ThirdParty.Test",
		"authorised.example._report._dmarc.thirdparty.test.",
	} {
		if _, _, err := r.ExchangeFrom(context.Background(), name, dns.TypeTXT); err != nil {
			t.Errorf("ExchangeFrom(%q) unexpectedly refused: %v", name, err)
		}
	}
	if got := inner.count(); got != 4 {
		t.Fatalf("queries reaching the transport = %d, want 4", got)
	}
}

// TestReportAuthorisationDoesNotWidenScope is the boundary on the exemption
// above. The authorised domain must be the whole prefix, not merely present
// somewhere in the name — otherwise the carve-out becomes a way through the
// guard for any name at all, which is precisely what it exists to prevent.
func TestReportAuthorisationDoesNotWidenScope(t *testing.T) {
	inner := &countingResolver{}
	r := NewScopedResolver(inner, NewScope([]string{"authorised.example"}, nil))

	ctx := context.Background()
	refused := []string{
		// A different domain borrowing the infix.
		"attacker.test._report._dmarc.elsewhere.test",
		// The authorised name appears, but not as the subject of the record.
		"attacker.test._report._dmarc.authorised.example.evil.test",
		// A near-miss registration in the subject position.
		"notauthorised.example._report._dmarc.thirdparty.test",
		// The infix present but incomplete.
		"authorised.example._report.thirdparty.test",
		// The infix as a suffix rather than a separator.
		"authorised.example._report._dmarc",
	}

	for _, name := range refused {
		if _, _, err := r.ExchangeFrom(ctx, name, dns.TypeTXT); !errors.Is(err, vantage.ErrOutOfScope) {
			t.Errorf("ExchangeFrom(%q) error = %v, want ErrOutOfScope", name, err)
		}
	}
	if got := inner.count(); got != 0 {
		t.Fatalf("%d queries reached the transport: %v", got, inner.queried)
	}
}

// TestFollowingAReferenceOutOfScopeIsRefused covers the case the transport
// guard exists for. Assessment follows references it discovers — a CNAME to a
// third party is the classic one — so the name queried was never in the
// caller's request and could not have been filtered up front.
func TestFollowingAReferenceOutOfScopeIsRefused(t *testing.T) {
	inner := &countingResolver{}
	r := NewScopedResolver(inner, NewScope([]string{"authorised.example"}, nil))

	// The alias itself is in scope and is resolved.
	if _, _, err := r.ExchangeFrom(context.Background(), "www.authorised.example", dns.TypeCNAME); err != nil {
		t.Fatalf("resolving the in-scope alias: %v", err)
	}
	// Its target is somebody else's infrastructure, and must not be followed.
	_, _, err := r.ExchangeFrom(context.Background(), "bucket.provider.test", dns.TypeA)
	if !errors.Is(err, vantage.ErrOutOfScope) {
		t.Fatalf("following the reference: error = %v, want ErrOutOfScope", err)
	}
	if got := inner.count(); got != 1 {
		t.Fatalf("queries reaching the transport = %d, want 1 (the in-scope alias only)", got)
	}
}

// TestEmptyScopeRefusesEverything pins the fail-closed default. A scope nobody
// configured must authorise nothing, not everything.
func TestEmptyScopeRefusesEverything(t *testing.T) {
	inner := &countingResolver{}
	r := NewScopedResolver(inner, NewScope(nil, nil))

	if _, _, err := r.ExchangeFrom(context.Background(), "anything.example", dns.TypeA); !errors.Is(err, vantage.ErrOutOfScope) {
		t.Fatalf("error = %v, want ErrOutOfScope", err)
	}
	if got := inner.count(); got != 0 {
		t.Fatalf("an unconfigured scope permitted %d queries", got)
	}
}

// TestUnlistedThirdPartyEndpointIsRefused covers the separation between target
// scope and third-party consent. Authorising assessment of a domain is not
// authorising disclosure of it to a certificate transparency log.
func TestUnlistedThirdPartyEndpointIsRefused(t *testing.T) {
	var reached int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewScopedHTTPClient(srv.Client(), NewScope([]string{"authorised.example"}, nil))

	req, err := http.NewRequest(http.MethodGet, "https://crt.sh/?q=authorised.example", nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	resp, err := c.Do(req)
	if !errors.Is(err, vantage.ErrOutOfScope) {
		t.Fatalf("error = %v, want ErrOutOfScope", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
		t.Error("a refused request must not yield a response")
	}
	if reached != 0 {
		t.Error("a refused request reached the network")
	}
}

// TestConsentedThirdPartyEndpointIsPermitted shows consent is what unlocks it,
// and that consent is given per endpoint rather than wholesale.
func TestConsentedThirdPartyEndpointIsPermitted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	c := NewScopedHTTPClient(srv.Client(),
		NewScope([]string{"authorised.example"}, []string{host}))

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("a consented endpoint was refused: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// TestHTTPToOutOfScopeTargetIsRefused guards the MTA-STS and takeover paths,
// which fetch over HTTP from names discovered during assessment.
func TestHTTPToOutOfScopeTargetIsRefused(t *testing.T) {
	c := NewScopedHTTPClient(nil, NewScope([]string{"authorised.example"}, nil))

	req, err := http.NewRequest(http.MethodGet,
		"https://mta-sts.forbidden.example/.well-known/mta-sts.txt", nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	if _, err := c.Do(req); !errors.Is(err, vantage.ErrOutOfScope) {
		t.Fatalf("error = %v, want ErrOutOfScope", err)
	}
	if len(c.Denials()) != 1 {
		t.Fatalf("denials recorded = %d, want 1", len(c.Denials()))
	}
	if c.Denials()[0].Kind != "http" {
		t.Errorf("denial kind = %q, want http", c.Denials()[0].Kind)
	}
}

// TestRefusalIsRecognisedAsRefusedOutcome closes the loop: a scope refusal must
// travel all the way through to an outcome the caller can act on, rather than
// being flattened into a generic failure.
func TestRefusalIsRecognisedAsRefusedOutcome(t *testing.T) {
	err := (&scopeGuard{}).deny("dns", "forbidden.example")
	if got := classifyOutcome(err); got != OutcomeRefused {
		t.Fatalf("outcome = %q, want %q", got, OutcomeRefused)
	}
}

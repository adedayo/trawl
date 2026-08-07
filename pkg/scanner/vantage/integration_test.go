package vantage_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"

	vantagepkg "github.com/adedayo/vantage/pkg"
	vaudit "github.com/adedayo/vantage/pkg/audit"

	tv "github.com/adedayo/trawl/pkg/scanner/vantage"
)

// These tests drive real vantage checks through Trawl's guarded transports
// against local servers.
//
// Everything else in this package fakes the assessor, which is right for
// testing translation but leaves the load-bearing claim untested: that an
// out-of-scope target emits no packet. That claim spans two codebases, and the
// combination — real checks, guarded transport — is precisely where a defect
// hides. A panic in vantage's MTA-STS fetcher, from an unchecked type
// assertion that held for its own client and failed for any wrapper, survived
// both test suites and was caught by a linter. Nothing here would have let it
// through.

// dnsRecorder is an authoritative-ish server that records every question it is
// asked, so a test can assert on packets that actually arrived rather than on
// the tool's stated intentions.
type dnsRecorder struct {
	mu       atomic.Pointer[[]string]
	asked    chan string
	handlers map[string][]dns.RR
}

func newDNSRecorder() *dnsRecorder {
	return &dnsRecorder{asked: make(chan string, 512), handlers: map[string][]dns.RR{}}
}

func (d *dnsRecorder) answer(name string, rrs ...dns.RR) {
	d.handlers[dns.Fqdn(name)] = append(d.handlers[dns.Fqdn(name)], rrs...)
}

func (d *dnsRecorder) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	for _, q := range r.Question {
		select {
		case d.asked <- strings.ToLower(q.Name):
		default:
		}
		for _, rr := range d.handlers[strings.ToLower(q.Name)] {
			if rr.Header().Rrtype == q.Qtype {
				m.Answer = append(m.Answer, rr)
			}
		}
	}
	_ = w.WriteMsg(m)
}

// names drains everything asked so far.
func (d *dnsRecorder) names() map[string]int {
	out := map[string]int{}
	for {
		select {
		case n := <-d.asked:
			out[strings.TrimSuffix(n, ".")]++
		default:
			return out
		}
	}
}

func startDNS(t *testing.T, h dns.Handler) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &dns.Server{PacketConn: pc, Net: "udp", Handler: h}
	go func() { _ = srv.ActivateAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown() })
	return pc.LocalAddr().String()
}

func mustRR(t *testing.T, s string) dns.RR {
	t.Helper()
	rr, err := dns.NewRR(s)
	if err != nil {
		t.Fatalf("bad RR %q: %v", s, err)
	}
	return rr
}

// zone stands up a DNS server that answers for in-scope.test and, deliberately,
// for out-of-scope.test too. The server being willing is the point: if a name
// is missing from the recorder afterwards, it is because the guard refused it,
// not because there was nothing to answer.
func zone(t *testing.T) (*dnsRecorder, string) {
	t.Helper()
	rec := newDNSRecorder()

	rec.answer("in-scope.test",
		mustRR(t, `in-scope.test. 300 IN TXT "v=spf1 include:out-of-scope.test -all"`),
		mustRR(t, "in-scope.test. 300 IN MX 10 mail.out-of-scope.test."),
		mustRR(t, "in-scope.test. 300 IN NS ns1.out-of-scope.test."),
		mustRR(t, "in-scope.test. 300 IN A 203.0.113.10"),
	)
	rec.answer("_dmarc.in-scope.test",
		mustRR(t, `_dmarc.in-scope.test. 300 IN TXT "v=DMARC1; p=reject; rua=mailto:d@in-scope.test"`))
	rec.answer("_mta-sts.in-scope.test",
		mustRR(t, `_mta-sts.in-scope.test. 300 IN TXT "v=STSv1; id=20260101000000"`))

	// The references above all point here. A check that follows one must be
	// refused at the transport.
	rec.answer("out-of-scope.test",
		mustRR(t, `out-of-scope.test. 300 IN TXT "v=spf1 -all"`),
		mustRR(t, "out-of-scope.test. 300 IN A 198.51.100.10"),
	)
	rec.answer("mail.out-of-scope.test",
		mustRR(t, "mail.out-of-scope.test. 300 IN A 198.51.100.11"))
	rec.answer("ns1.out-of-scope.test",
		mustRR(t, "ns1.out-of-scope.test. 300 IN A 198.51.100.12"))

	return rec, startDNS(t, rec)
}

func clientFor(addr string) vantagepkg.Resolver {
	return vantagepkg.NewClient(vantagepkg.Config{
		Servers:      []string{addr},
		QueryTimeout: 500 * time.Millisecond,
		TotalTimeout: 2 * time.Second,
	})
}

// TestGuardedAssessmentEmitsNoOutOfScopePackets is the claim the whole design
// rests on, tested end to end: real checks, a real resolver, a server willing
// to answer for both zones, and a scope permitting only one.
func TestGuardedAssessmentEmitsNoOutOfScopePackets(t *testing.T) {
	rec, addr := zone(t)

	scope := tv.NewScope([]string{"in-scope.test"}, nil)
	guardedDNS := tv.NewScopedResolver(clientFor(addr), scope)
	guardedHTTP := tv.NewScopedHTTPClient(nil, scope)

	assessor, err := vaudit.NewAssessor(guardedDNS,
		vaudit.WithHTTPClient(guardedHTTP),
		vaudit.WithVersion("integration-test"))
	if err != nil {
		t.Fatalf("NewAssessor: %v", err)
	}

	adapter, err := tv.New(assessor)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := adapter.Assess(context.Background(), tv.Request{
		AssetID: "asset-1",
		Domain:  "in-scope.test",
		Profile: "standard",
	})
	// A refused reference is recorded against its check, not raised as a run
	// failure, so the assessment as a whole must still conclude.
	if err != nil {
		t.Fatalf("Assess returned a run failure: %v", err)
	}

	asked := rec.names()

	for name, count := range asked {
		if !strings.HasSuffix(name, "in-scope.test") || strings.HasSuffix(name, "out-of-scope.test") {
			t.Errorf("a packet left for %q (%d times) — it is outside the authorised scope",
				name, count)
		}
	}

	// The guard must not have achieved silence by refusing everything: an
	// assessment that emitted nothing at all would pass the assertion above
	// while measuring nothing.
	if len(asked) == 0 {
		t.Fatal("no queries were made at all; the test proves nothing")
	}
	if asked["in-scope.test"] == 0 {
		t.Error("the authorised target itself was never queried")
	}

	// Following the SPF include, the MX and the NS all lead out of scope, so
	// the guard must have recorded refusals rather than passing them through.
	if len(guardedDNS.Denials()) == 0 {
		t.Error("expected refusals: the zone deliberately references another domain")
	}
	for _, d := range guardedDNS.Denials() {
		if !strings.HasSuffix(d.Name, "out-of-scope.test") {
			t.Errorf("refused %q, which was in scope and should have been permitted", d.Name)
		}
	}

	if len(res.Coverage) == 0 {
		t.Error("an assessment that ran must report coverage")
	}
}

// TestGuardedAssessmentSurvivesEveryCheck runs the deep profile, which reaches
// the MTA-STS and takeover paths that fetch over HTTP.
//
// The assertion is simply that nothing panics. That is a low bar and it is
// exactly the bar that was previously failed: vantage asserted its resolver
// into an interface the contract does not require, which held for its own
// client and crashed for any wrapper — so the first consumer to guard a
// resolver would have brought the process down.
func TestGuardedAssessmentSurvivesEveryCheck(t *testing.T) {
	_, addr := zone(t)

	scope := tv.NewScope([]string{"in-scope.test"}, nil)
	guardedDNS := tv.NewScopedResolver(clientFor(addr), scope)
	guardedHTTP := tv.NewScopedHTTPClient(nil, scope)

	assessor, err := vaudit.NewAssessor(guardedDNS, vaudit.WithHTTPClient(guardedHTTP))
	if err != nil {
		t.Fatalf("NewAssessor: %v", err)
	}
	adapter, err := tv.New(assessor)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Panics in a check goroutine would take the process down rather than fail
	// this test politely, which is itself the finding: a library that panics
	// on a documented-interface implementation is not embeddable.
	res, err := adapter.Assess(ctx, tv.Request{
		AssetID: "asset-1",
		Domain:  "in-scope.test",
		Profile: "deep",
		Hosts:   []string{"www.in-scope.test"},
	})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if res.Outcome == "" {
		t.Error("every assessment must carry an outcome")
	}
}

// TestUnconsentedThirdPartyEndpointIsRefusedInPractice covers the HTTP
// boundary with a real server. Consent to assess a domain is not consent to
// disclose it to whatever service a check happens to query.
func TestUnconsentedThirdPartyEndpointIsRefusedInPractice(t *testing.T) {
	var reached atomic.Int64
	third := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer third.Close()

	scope := tv.NewScope([]string{"in-scope.test"}, nil)
	guardedHTTP := tv.NewScopedHTTPClient(nil, scope)

	req, err := http.NewRequest(http.MethodGet, third.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if _, err := guardedHTTP.Do(req); err == nil {
		t.Fatal("expected the request to be refused")
	}

	if n := reached.Load(); n != 0 {
		t.Errorf("the third-party server was contacted %d times; it should never have been reached", n)
	}
	if len(guardedHTTP.Denials()) != 1 {
		t.Errorf("recorded %d denials, want 1 — a silent refusal is as bad as a leak",
			len(guardedHTTP.Denials()))
	}
}

// TestConsentedThirdPartyEndpointIsReachedInPractice is the other half: an
// endpoint the operator listed must actually be reachable, or consent would be
// meaningless and checks would fail for the wrong reason.
func TestConsentedThirdPartyEndpointIsReachedInPractice(t *testing.T) {
	var reached atomic.Int64
	third := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer third.Close()

	host, _, err := net.SplitHostPort(strings.TrimPrefix(third.URL, "http://"))
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}

	scope := tv.NewScope([]string{"in-scope.test"}, []string{host})
	guardedHTTP := tv.NewScopedHTTPClient(nil, scope)

	req, err := http.NewRequest(http.MethodGet, third.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := guardedHTTP.Do(req)
	if err != nil {
		t.Fatalf("a consented endpoint must be reachable: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test

	if reached.Load() != 1 {
		t.Errorf("the consented server was reached %d times, want 1", reached.Load())
	}
	if len(guardedHTTP.Denials()) != 0 {
		t.Errorf("a consented endpoint must not be recorded as denied, got %v",
			guardedHTTP.Denials())
	}
}

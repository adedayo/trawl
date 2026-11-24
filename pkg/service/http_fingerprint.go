package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/software"
)

const (
	fingerprintTimeout      = 5 * time.Second
	fingerprintMaxBodyBytes = 64 * 1024
	fingerprintMaxRedirects = 2
)

// HTTPFingerprinter performs one bounded, read-only HTTP GET against an
// already-confirmed responding service to extract headers, cookies, and HTML
// generator/framework fingerprints.
type HTTPFingerprinter func(ctx context.Context, scope *vadapter.Scope, scheme, host string, port uint16) ([]software.ParsedTech, error)

// defaultHTTPFingerprint issues a single non-destructive GET / request against
// an in-scope host, refusing any target or redirect outside the authorised
// scope. It reads at most 64 KiB of the response body.
func defaultHTTPFingerprint(ctx context.Context, scope *vadapter.Scope, scheme, host string, port uint16) ([]software.ParsedTech, error) {
	if scope == nil || !scope.PermitsTarget(host) {
		return nil, fmt.Errorf("fingerprint: %q is outside the authorised scope", host)
	}
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("fingerprint: unsupported scheme %q", scheme)
	}

	reqCtx, cancel := context.WithTimeout(ctx, fingerprintTimeout)
	defer cancel()

	// TLS certificate validity is assessed separately by Vantage; allowing
	// self-signed or expired certs here ensures we still fingerprint exposed
	// admin panels and staging hosts that present untrusted certificates.
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: fingerprintTimeout}).DialContext,
		TLSHandshakeTimeout: fingerprintTimeout,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // read-only external fingerprinting
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   fingerprintTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= fingerprintMaxRedirects {
				return http.ErrUseLastResponse
			}
			if !scope.PermitsTarget(req.URL.Hostname()) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	targetURL := (&url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, fmt.Sprint(port)),
		Path:   "/",
	}).String()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Trawl-External-Assessment/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, fingerprintMaxBodyBytes))
	return software.FingerprintHTTPResponse(resp.Header, body), nil
}

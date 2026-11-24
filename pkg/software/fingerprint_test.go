package software_test

import (
	"net/http"
	"testing"

	"github.com/adedayo/trawl/pkg/software"
)

func TestFingerprintHTTPResponseExtractsHeadersCookiesMetaAndMarkers(t *testing.T) {
	headers := http.Header{}
	headers.Add("X-AspNet-Version", "4.0.30319")
	headers.Add("X-Jenkins", "2.440.1")
	headers.Add("CF-Ray", "8a1b2c3d-LHR")
	headers.Add("Set-Cookie", "PHPSESSID=abc123; Path=/; HttpOnly")
	headers.Add("Set-Cookie", "laravel_session=xyz; Secure")

	body := []byte(`<!doctype html>
<html>
  <head>
    <meta name="generator" content="WordPress 6.5.2" />
    <meta name="generator" content="Drupal 10 (https://www.drupal.org)" />
  </head>
  <body>
    <script id="__NEXT_DATA__" type="application/json">{}</script>
  </body>
</html>`)

	got := software.FingerprintHTTPResponse(headers, body)
	find := func(name, version string, known bool) bool {
		for _, item := range got {
			if item.Name == name && item.Version == version && item.VersionKnown == known {
				return true
			}
		}
		return false
	}

	checks := []struct {
		name    string
		version string
		known   bool
	}{
		{"ASP.NET", "4.0.30319", true},
		{"Jenkins", "2.440.1", true},
		{"Cloudflare", "", false},
		{"PHP", "", false},
		{"Laravel", "", false},
		{"WordPress", "6.5.2", true},
		{"Drupal", "10", true},
		{"Next.js", "", false},
	}
	for _, want := range checks {
		if !find(want.name, want.version, want.known) {
			t.Errorf("missing expected fingerprint %+v in %+v", want, got)
		}
	}
}

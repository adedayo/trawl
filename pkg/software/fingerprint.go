package software

import (
	"net/http"
	"regexp"
	"strings"
)

var (
	metaGeneratorTagRe = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	metaNameGenRe      = regexp.MustCompile(`(?is)\bname\s*=\s*["']?generator["']?`)
	metaContentRe      = regexp.MustCompile(`(?is)\bcontent\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	wpVerQueryRe       = regexp.MustCompile(`(?i)/wp-(?:includes|content)/[^"'?\s>]+\?ver=([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
)

// FingerprintHTTPResponse extracts software and technology observations from
// an HTTP response's headers, Set-Cookie names, HTML <meta name="generator">
// tags, and passive HTML framework markers.
//
// It never invents a version when the response does not state one, preserves
// the raw evidence string for auditability, and deduplicates by (name, version).
func FingerprintHTTPResponse(headers http.Header, body []byte) []ParsedTech {
	var out []ParsedTech
	seen := map[string]bool{}
	add := func(item ParsedTech) {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return
		}
		ver := strings.TrimSpace(item.Version)
		known := item.VersionKnown && ver != "" && versionToken.MatchString(ver)
		if !known {
			ver = ""
		}
		key := strings.ToLower(name) + "\x00" + ver
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, ParsedTech{
			Raw:          item.Raw,
			Name:         name,
			Version:      ver,
			VersionKnown: known,
		})
	}
	addHeaderTech := func(headerName string, values []string) {
		for _, v := range values {
			for _, parsed := range ParseServerHeader(v) {
				parsed.Raw = headerName + ": " + strings.TrimSpace(v)
				add(parsed)
			}
		}
	}

	if headers != nil {
		// Standard and common version-bearing headers.
		addHeaderTech("Server", headers.Values("Server"))
		addHeaderTech("X-Powered-By", headers.Values("X-Powered-By"))
		addHeaderTech("X-Generator", headers.Values("X-Generator"))
		addHeaderTech("X-Powered-CMS", headers.Values("X-Powered-CMS"))

		if v := strings.TrimSpace(headers.Get("X-AspNet-Version")); v != "" {
			add(ParsedTech{Raw: "X-AspNet-Version: " + v, Name: "ASP.NET", Version: v, VersionKnown: true})
		}
		if v := strings.TrimSpace(headers.Get("X-AspNetMvc-Version")); v != "" {
			add(ParsedTech{Raw: "X-AspNetMvc-Version: " + v, Name: "ASP.NET MVC", Version: v, VersionKnown: true})
		}
		if v := strings.TrimSpace(headers.Get("X-Jenkins")); v != "" {
			add(ParsedTech{Raw: "X-Jenkins: " + v, Name: "Jenkins", Version: v, VersionKnown: true})
		}
		if v := strings.TrimSpace(headers.Get("Kbn-Version")); v != "" {
			add(ParsedTech{Raw: "Kbn-Version: " + v, Name: "Kibana", Version: v, VersionKnown: true})
		} else if strings.TrimSpace(headers.Get("Kbn-Name")) != "" {
			add(ParsedTech{Raw: "Kbn-Name: " + headers.Get("Kbn-Name"), Name: "Kibana"})
		}

		// Proxy / CDN / platform headers (version unknown, still valuable intelligence).
		if strings.TrimSpace(headers.Get("CF-Ray")) != "" {
			add(ParsedTech{Raw: "CF-Ray header", Name: "Cloudflare"})
		}
		if strings.TrimSpace(headers.Get("X-Amz-Cf-Id")) != "" {
			add(ParsedTech{Raw: "X-Amz-Cf-Id header", Name: "Amazon CloudFront"})
		}
		if strings.TrimSpace(headers.Get("X-Azure-Ref")) != "" {
			add(ParsedTech{Raw: "X-Azure-Ref header", Name: "Azure Front Door"})
		}
		if strings.TrimSpace(headers.Get("X-Vercel-Id")) != "" {
			add(ParsedTech{Raw: "X-Vercel-Id header", Name: "Vercel"})
		}
		if strings.TrimSpace(headers.Get("X-Netlify-Request-Id")) != "" {
			add(ParsedTech{Raw: "X-Netlify-Request-Id header", Name: "Netlify"})
		}
		if strings.TrimSpace(headers.Get("X-GitHub-Request-Id")) != "" {
			add(ParsedTech{Raw: "X-GitHub-Request-Id header", Name: "GitHub Pages"})
		}
		if strings.TrimSpace(headers.Get("X-Shopify-Stage")) != "" {
			add(ParsedTech{Raw: "X-Shopify-Stage header", Name: "Shopify"})
		}
		if strings.TrimSpace(headers.Get("X-Drupal-Cache")) != "" || strings.TrimSpace(headers.Get("X-Drupal-Dynamic-Cache")) != "" {
			add(ParsedTech{Raw: "X-Drupal-Cache header", Name: "Drupal"})
		}
		if strings.TrimSpace(headers.Get("X-Varnish")) != "" {
			add(ParsedTech{Raw: "X-Varnish header", Name: "Varnish"})
		}
		if via := strings.TrimSpace(headers.Get("Via")); via != "" {
			lowerVia := strings.ToLower(via)
			switch {
			case strings.Contains(lowerVia, "cloudfront"):
				add(ParsedTech{Raw: "Via: " + via, Name: "Amazon CloudFront"})
			case strings.Contains(lowerVia, "varnish"):
				add(ParsedTech{Raw: "Via: " + via, Name: "Varnish"})
			case strings.Contains(lowerVia, "envoy"):
				add(ParsedTech{Raw: "Via: " + via, Name: "Envoy"})
			case strings.Contains(lowerVia, "fastly"):
				add(ParsedTech{Raw: "Via: " + via, Name: "Fastly"})
			}
		}

		// Set-Cookie session identifiers.
		for _, cookie := range headers.Values("Set-Cookie") {
			name, _, _ := strings.Cut(cookie, "=")
			lowerName := strings.ToLower(strings.TrimSpace(name))
			switch {
			case lowerName == "phpsessid":
				add(ParsedTech{Raw: "Set-Cookie: PHPSESSID", Name: "PHP"})
			case lowerName == "jsessionid":
				add(ParsedTech{Raw: "Set-Cookie: JSESSIONID", Name: "Java Servlet"})
			case lowerName == "asp.net_sessionid" || lowerName == ".aspxauth":
				add(ParsedTech{Raw: "Set-Cookie: " + strings.TrimSpace(name), Name: "ASP.NET"})
			case lowerName == "laravel_session":
				add(ParsedTech{Raw: "Set-Cookie: laravel_session", Name: "Laravel"})
			case strings.HasPrefix(lowerName, "wordpress_") || strings.HasPrefix(lowerName, "wp-settings"):
				add(ParsedTech{Raw: "Set-Cookie: " + strings.TrimSpace(name), Name: "WordPress"})
			case lowerName == "connect.sid":
				add(ParsedTech{Raw: "Set-Cookie: connect.sid", Name: "Express"})
			case lowerName == "grafana_session":
				add(ParsedTech{Raw: "Set-Cookie: grafana_session", Name: "Grafana"})
			case lowerName == "_gitlab_session":
				add(ParsedTech{Raw: "Set-Cookie: _gitlab_session", Name: "GitLab"})
			}
		}
	}

	if len(body) > 0 {
		html := string(body)

		// 1. <meta name="generator" content="...">
		for _, tag := range metaGeneratorTagRe.FindAllString(html, 20) {
			if !metaNameGenRe.MatchString(tag) {
				continue
			}
			m := metaContentRe.FindStringSubmatch(tag)
			if len(m) == 0 {
				continue
			}
			content := m[1]
			if content == "" {
				content = m[2]
			}
			if content == "" {
				content = m[3]
			}
			content = strings.TrimSpace(content)
			if content == "" {
				continue
			}
			// Strip parenthetical URLs like "Drupal 10 (https://www.drupal.org)".
			cleaned := content
			if idx := strings.Index(cleaned, "("); idx > 0 {
				cleaned = strings.TrimSpace(cleaned[:idx])
			}
			cleaned = strings.TrimSuffix(cleaned, "!")
			// Handle "Drupal 10" (single major version after space) as well as dotted versions.
			parts := strings.Fields(cleaned)
			if len(parts) == 2 && versionToken.MatchString(parts[1]) {
				add(ParsedTech{
					Raw:          "meta generator: " + content,
					Name:         strings.TrimSuffix(parts[0], "!"),
					Version:      parts[1],
					VersionKnown: true,
				})
				continue
			}
			for _, parsed := range ParseTech(cleaned) {
				parsed.Raw = "meta generator: " + content
				add(parsed)
			}
		}

		// 2. Well-known passive HTML markers.
		lowerHTML := strings.ToLower(html)
		if strings.Contains(lowerHTML, "/wp-content/") || strings.Contains(lowerHTML, "/wp-includes/") {
			ver := ""
			if m := wpVerQueryRe.FindStringSubmatch(html); len(m) == 2 {
				ver = m[1]
			}
			add(ParsedTech{Raw: "HTML marker: /wp-content/", Name: "WordPress", Version: ver, VersionKnown: ver != ""})
		}
		if strings.Contains(lowerHTML, "/_next/static/") || strings.Contains(html, "__NEXT_DATA__") {
			add(ParsedTech{Raw: "HTML marker: __NEXT_DATA__", Name: "Next.js"})
		}
		if strings.Contains(lowerHTML, "/_nuxt/") || strings.Contains(html, "__NUXT__") {
			add(ParsedTech{Raw: "HTML marker: __NUXT__", Name: "Nuxt.js"})
		}
		if strings.Contains(lowerHTML, "drupalsettings") || strings.Contains(lowerHTML, "data-drupal-selector") {
			add(ParsedTech{Raw: "HTML marker: drupalSettings", Name: "Drupal"})
		}
		if strings.Contains(lowerHTML, "cdn.shopify.com") {
			add(ParsedTech{Raw: "HTML marker: cdn.shopify.com", Name: "Shopify"})
		}
		if strings.Contains(html, "grafanaBootData") {
			add(ParsedTech{Raw: "HTML marker: grafanaBootData", Name: "Grafana"})
		}
		if strings.Contains(html, "gon.gitlab_url") {
			add(ParsedTech{Raw: "HTML marker: gon.gitlab_url", Name: "GitLab"})
		}
	}

	return out
}

// Package software parses observed technology strings into structured product
// and version records and resolves them against the curated product map.
//
// It never invents a version when none was observed, and never infers a CPE
// identity from free text at runtime.
package software

import (
	"regexp"
	"strings"
)

// ParsedTech is the deterministic parse of one raw technology string (such as
// an httpx -tech-detect entry like "Nginx:1.18.0" or "WordPress").
type ParsedTech struct {
	Raw          string
	Name         string
	Version      string
	VersionKnown bool
}

// versionToken matches a plausible software version token (starts with a digit
// and contains digits, dots, hyphens, underscores, pluses, or short alphanumeric
// suffixes like 1.1.1k, 2.4.49-rc1, 8.1.2+ubuntu0.22.04.1).
var versionToken = regexp.MustCompile(`^[0-9][0-9A-Za-z._+-]*$`)

// ParseTech splits a raw technology string into its observed product name and
// optional version.
//
// Supported forms:
//   - "Name:Version" (httpx's canonical output, e.g. "Nginx:1.18.0")
//   - "Name/Version" (banner-style, e.g. "Apache/2.4.49"), except protocol names
//     such as "HTTP/2" or "HTTP/3" where the token after "/" is a single-digit
//     protocol revision with no dot and the prefix is "HTTP"
//   - "Name Version" when the trailing whitespace-separated token starts with a
//     digit and contains a dot (e.g. "Apache HTTP Server 2.4.49")
//   - Multiple versions separated by "," or ";" (e.g. "jQuery:3.5.1,3.6.0") emit
//     one ParsedTech per distinct version, preserving the raw string.
//
// If no version is present, VersionKnown is false and Version is empty.
func ParseTech(raw string) []ParsedTech {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	// 1. Colon separator (httpx's standard format: "Name:Version").
	if idx := strings.LastIndex(trimmed, ":"); idx > 0 {
		name := strings.TrimSpace(trimmed[:idx])
		verPart := strings.TrimSpace(trimmed[idx+1:])
		if name != "" {
			if verPart == "" {
				return []ParsedTech{{Raw: trimmed, Name: name}}
			}
			if versions := splitVersions(verPart); len(versions) > 0 {
				out := make([]ParsedTech, 0, len(versions))
				for _, v := range versions {
					out = append(out, ParsedTech{
						Raw:          trimmed,
						Name:         name,
						Version:      v,
						VersionKnown: true,
					})
				}
				return out
			}
		}
	}

	// 2. Slash separator ("Apache/2.4.49"), preserving protocol tokens like HTTP/2 or HTTP/3.
	if idx := strings.LastIndex(trimmed, "/"); idx > 0 {
		name := strings.TrimSpace(trimmed[:idx])
		verPart := strings.TrimSpace(trimmed[idx+1:])
		if name != "" && !strings.EqualFold(name, "http") && verPart != "" {
			if versions := splitVersions(verPart); len(versions) > 0 {
				out := make([]ParsedTech, 0, len(versions))
				for _, v := range versions {
					out = append(out, ParsedTech{
						Raw:          trimmed,
						Name:         name,
						Version:      v,
						VersionKnown: true,
					})
				}
				return out
			}
		}
	}

	// 3. Space-separated trailing dotted version ("Apache HTTP Server 2.4.49").
	if idx := strings.LastIndex(trimmed, " "); idx > 0 {
		name := strings.TrimSpace(trimmed[:idx])
		verPart := strings.TrimSpace(trimmed[idx+1:])
		if name != "" && strings.Contains(verPart, ".") && versionToken.MatchString(verPart) {
			return []ParsedTech{{
				Raw:          trimmed,
				Name:         name,
				Version:      verPart,
				VersionKnown: true,
			}}
		}
	}

	// 4. Bare name, version unknown.
	return []ParsedTech{{
		Raw:          trimmed,
		Name:         trimmed,
		Version:      "",
		VersionKnown: false,
	}}
}

// ParseServerHeader parses an HTTP Server header (RFC 9110 product tokens and
// optional parenthetical comments, e.g. "Apache/2.4.49 (Ubuntu) OpenSSL/1.1.1k")
// into structured ParsedTech items, preserving the full header as Raw and never
// inventing a version when none is present.
func ParseServerHeader(raw string) []ParsedTech {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var tokens []string
	var cur strings.Builder
	inParen := false
	for _, r := range trimmed {
		switch {
		case r == '(' && !inParen:
			if tok := strings.TrimSpace(cur.String()); tok != "" {
				tokens = append(tokens, tok)
			}
			cur.Reset()
			inParen = true
		case r == ')' && inParen:
			if comment := strings.TrimSpace(cur.String()); comment != "" {
				// Split comment tokens such as "(Ubuntu)" or "(Debian/Linux)".
				for _, part := range strings.FieldsFunc(comment, func(c rune) bool {
					return c == ' ' || c == ';' || c == ','
				}) {
					if p := strings.TrimSpace(part); p != "" {
						tokens = append(tokens, p)
					}
				}
			}
			cur.Reset()
			inParen = false
		case !inParen && (r == ' ' || r == '\t'):
			if tok := strings.TrimSpace(cur.String()); tok != "" {
				tokens = append(tokens, tok)
			}
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if tok := strings.TrimSpace(cur.String()); tok != "" {
		tokens = append(tokens, tok)
	}

	seen := map[string]bool{}
	var out []ParsedTech
	for _, tok := range tokens {
		for _, parsed := range ParseTech(tok) {
			parsed.Raw = trimmed
			key := strings.ToLower(parsed.Name) + "\x00" + parsed.Version
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, parsed)
		}
	}
	return out
}

func splitVersions(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';'
	})
	var out []string
	seen := map[string]bool{}
	for _, p := range parts {
		v := strings.TrimSpace(p)
		v = strings.TrimPrefix(v, "v")
		v = strings.TrimPrefix(v, "V")
		if v == "" || !versionToken.MatchString(v) {
			return nil
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

var (
	sshBannerRe     = regexp.MustCompile(`(?i)^SSH-[0-9.]+-([A-Za-z][A-Za-z0-9.-]*)[_-]([0-9][0-9A-Za-z._+-]*)`)
	knownBannerRe   = regexp.MustCompile(`(?i)\b(Postfix|Exim|Sendmail|Dovecot|Cyrus IMAP|vsFTPd|ProFTPD|Pure-FTPd|OpenSSH|Dropbear|MySQL|MariaDB|PostgreSQL|Redis)(?:[/ _]+([0-9][0-9A-Za-z._+-]*))?`)
	osCommentHintRe = regexp.MustCompile(`(?i)\b(Ubuntu|Debian)\b`)
)

// ParseServiceBanner extracts structured product/version observations from a
// non-HTTP service greeting banner (such as SSH, SMTP, FTP, IMAP, or POP3).
// It preserves the full banner as Raw and never invents a version when the
// banner does not state one.
func ParseServiceBanner(protocol, raw string) []ParsedTech {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var out []ParsedTech
	seen := map[string]bool{}
	add := func(name, version string) {
		name = strings.TrimSpace(name)
		version = strings.TrimSpace(version)
		if name == "" {
			return
		}
		known := version != "" && versionToken.MatchString(version)
		if !known {
			version = ""
		}
		key := strings.ToLower(name) + "\x00" + version
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, ParsedTech{
			Raw:          trimmed,
			Name:         name,
			Version:      version,
			VersionKnown: known,
		})
	}

	// 1. SSH identification string: "SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.1"
	if m := sshBannerRe.FindStringSubmatch(trimmed); len(m) == 3 {
		add(m[1], m[2])
		if om := osCommentHintRe.FindStringSubmatch(trimmed); len(om) == 2 {
			add(om[1], "")
		}
		return out
	}

	// 2. Known mail/FTP/database daemon banners.
	for _, m := range knownBannerRe.FindAllStringSubmatch(trimmed, -1) {
		add(m[1], m[2])
	}
	if len(out) > 0 {
		if om := osCommentHintRe.FindStringSubmatch(trimmed); len(om) == 2 {
			add(om[1], "")
		}
		return out
	}

	// 3. Fallback: try RFC product/version tokens (e.g. "MyDaemon/1.2.3").
	for _, parsed := range ParseServerHeader(trimmed) {
		if parsed.VersionKnown {
			add(parsed.Name, parsed.Version)
		}
	}
	if len(out) > 0 {
		return out
	}

	// 4. Keep the raw banner as an unversioned observation only if a protocol
	// name was known, without inventing a product or version.
	if p := strings.TrimSpace(protocol); p != "" {
		add(p, "")
	}
	return out
}

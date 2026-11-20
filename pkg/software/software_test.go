package software

import (
	"testing"
)

func TestParseTech(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []ParsedTech
	}{
		{
			name: "standard httpx Name:Version",
			raw:  "Nginx:1.18.0",
			want: []ParsedTech{{Raw: "Nginx:1.18.0", Name: "Nginx", Version: "1.18.0", VersionKnown: true}},
		},
		{
			name: "bare name without version",
			raw:  "WordPress",
			want: []ParsedTech{{Raw: "WordPress", Name: "WordPress", Version: "", VersionKnown: false}},
		},
		{
			name: "colon with empty version does not invent version",
			raw:  "Nginx:",
			want: []ParsedTech{{Raw: "Nginx:", Name: "Nginx", Version: "", VersionKnown: false}},
		},
		{
			name: "slash separator banner style",
			raw:  "Apache/2.4.49",
			want: []ParsedTech{{Raw: "Apache/2.4.49", Name: "Apache", Version: "2.4.49", VersionKnown: true}},
		},
		{
			name: "protocol HTTP/3 is not split as a version",
			raw:  "HTTP/3",
			want: []ParsedTech{{Raw: "HTTP/3", Name: "HTTP/3", Version: "", VersionKnown: false}},
		},
		{
			name: "space separated trailing dotted version",
			raw:  "Apache HTTP Server 2.4.49",
			want: []ParsedTech{{Raw: "Apache HTTP Server 2.4.49", Name: "Apache HTTP Server", Version: "2.4.49", VersionKnown: true}},
		},
		{
			name: "alphanumeric patch suffix",
			raw:  "OpenSSL:1.1.1k",
			want: []ParsedTech{{Raw: "OpenSSL:1.1.1k", Name: "OpenSSL", Version: "1.1.1k", VersionKnown: true}},
		},
		{
			name: "leading v stripped from version",
			raw:  "jQuery:v3.5.1",
			want: []ParsedTech{{Raw: "jQuery:v3.5.1", Name: "jQuery", Version: "3.5.1", VersionKnown: true}},
		},
		{
			name: "multiple versions separated by comma",
			raw:  "jQuery:3.5.1,3.6.0",
			want: []ParsedTech{
				{Raw: "jQuery:3.5.1,3.6.0", Name: "jQuery", Version: "3.5.1", VersionKnown: true},
				{Raw: "jQuery:3.5.1,3.6.0", Name: "jQuery", Version: "3.6.0", VersionKnown: true},
			},
		},
		{
			name: "non-version after colon kept as bare name",
			raw:  "AWS:CloudFront",
			want: []ParsedTech{{Raw: "AWS:CloudFront", Name: "AWS:CloudFront", Version: "", VersionKnown: false}},
		},
		{
			name: "whitespace only returns nil",
			raw:  "   ",
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseTech(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("ParseTech(%q) returned %d items %+v, want %d %+v", tc.raw, len(got), got, len(tc.want), tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("ParseTech(%q)[%d] = %+v, want %+v", tc.raw, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseServerHeader(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []ParsedTech
	}{
		{
			name: "nginx with version",
			raw:  "nginx/1.18.0",
			want: []ParsedTech{{Raw: "nginx/1.18.0", Name: "nginx", Version: "1.18.0", VersionKnown: true}},
		},
		{
			name: "apache with OS comment and OpenSSL",
			raw:  "Apache/2.4.49 (Ubuntu) OpenSSL/1.1.1k",
			want: []ParsedTech{
				{Raw: "Apache/2.4.49 (Ubuntu) OpenSSL/1.1.1k", Name: "Apache", Version: "2.4.49", VersionKnown: true},
				{Raw: "Apache/2.4.49 (Ubuntu) OpenSSL/1.1.1k", Name: "Ubuntu", Version: "", VersionKnown: false},
				{Raw: "Apache/2.4.49 (Ubuntu) OpenSSL/1.1.1k", Name: "OpenSSL", Version: "1.1.1k", VersionKnown: true},
			},
		},
		{
			name: "bare proxy header without version",
			raw:  "cloudflare",
			want: []ParsedTech{{Raw: "cloudflare", Name: "cloudflare", Version: "", VersionKnown: false}},
		},
		{
			name: "microsoft IIS with version",
			raw:  "Microsoft-IIS/10.0",
			want: []ParsedTech{{Raw: "Microsoft-IIS/10.0", Name: "Microsoft-IIS", Version: "10.0", VersionKnown: true}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseServerHeader(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("ParseServerHeader(%q) = %+v, want %+v", tc.raw, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("ParseServerHeader(%q)[%d] = %+v, want %+v", tc.raw, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseServiceBanner(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		raw      string
		want     []ParsedTech
	}{
		{
			name:     "OpenSSH banner with Ubuntu comment",
			protocol: "ssh",
			raw:      "SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.1",
			want: []ParsedTech{
				{Raw: "SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.1", Name: "OpenSSH", Version: "8.9p1", VersionKnown: true},
				{Raw: "SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.1", Name: "Ubuntu", Version: "", VersionKnown: false},
			},
		},
		{
			name:     "SMTP Postfix without version",
			protocol: "smtp",
			raw:      "220 mx.example.com ESMTP Postfix",
			want: []ParsedTech{
				{Raw: "220 mx.example.com ESMTP Postfix", Name: "Postfix", Version: "", VersionKnown: false},
			},
		},
		{
			name:     "SMTP Exim with version",
			protocol: "smtp",
			raw:      "220 mx.example.com ESMTP Exim 4.96",
			want: []ParsedTech{
				{Raw: "220 mx.example.com ESMTP Exim 4.96", Name: "Exim", Version: "4.96", VersionKnown: true},
			},
		},
		{
			name:     "FTP vsFTPd with version",
			protocol: "ftp",
			raw:      "220 (vsFTPd 3.0.5)",
			want: []ParsedTech{
				{Raw: "220 (vsFTPd 3.0.5)", Name: "vsFTPd", Version: "3.0.5", VersionKnown: true},
			},
		},
		{
			name:     "IMAP Dovecot without version",
			protocol: "imap",
			raw:      "* OK Dovecot ready.",
			want: []ParsedTech{
				{Raw: "* OK Dovecot ready.", Name: "Dovecot", Version: "", VersionKnown: false},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseServiceBanner(tc.protocol, tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("ParseServiceBanner(%q, %q) = %+v, want %+v", tc.protocol, tc.raw, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("ParseServiceBanner(%q, %q)[%d] = %+v, want %+v", tc.protocol, tc.raw, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestEmbeddedProductMapValidAndCoversPhase0Sample(t *testing.T) {
	m, err := LoadMap()
	if err != nil {
		t.Fatalf("LoadMap() failed: %v", err)
	}
	if m.Version() == "" {
		t.Fatal("product map version must not be empty")
	}

	// Every name observed in the Phase 0 sample must be either mapped or
	// explicitly listed as deliberately unmapped.
	phase0Sample := []string{
		"Nginx",
		"Apache HTTP Server",
		"PHP",
		"OpenSSL",
		"WordPress",
		"jQuery",
		"Cloudflare",
		"HSTS",
		"HTTP/3",
		"Google Font API",
	}
	for _, name := range phase0Sample {
		_, mapped := m.Lookup(name)
		unmapped := m.IsDeliberatelyUnmapped(name)
		if !mapped && !unmapped {
			t.Errorf("Phase 0 sample name %q is neither mapped nor listed in deliberatelyUnmapped", name)
		}
	}

	// Spot-check key mappings.
	nginx, ok := m.Lookup("Nginx")
	if !ok || nginx.Vendor != "f5" || nginx.Product != "nginx" || nginx.CPEPart != "a" {
		t.Errorf("Lookup(Nginx) = %+v, ok=%v; want f5:nginx (part a)", nginx, ok)
	}
	if _, ok := m.Lookup("Cloudflare"); ok {
		t.Error("Cloudflare should not be mapped to a CPE product")
	}
}

func TestLoadMapRejectsMalformedAndConflictingMaps(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{
			name: "unknown field rejected by strict decoding",
			json: `{"version":"1.0","extra":true,"entries":[{"names":["nginx"],"vendor":"f5","product":"nginx"}]}`,
		},
		{
			name: "missing vendor",
			json: `{"version":"1.0","entries":[{"names":["nginx"],"vendor":"","product":"nginx"}]}`,
		},
		{
			name: "missing product",
			json: `{"version":"1.0","entries":[{"names":["nginx"],"vendor":"f5","product":""}]}`,
		},
		{
			name: "conflicting collision",
			json: `{"version":"1.0","entries":[{"names":["nginx"],"vendor":"f5","product":"nginx"},{"names":["Nginx"],"vendor":"nginx","product":"nginx"}]}`,
		},
		{
			name: "mapped and deliberatelyUnmapped overlap",
			json: `{"version":"1.0","entries":[{"names":["nginx"],"vendor":"f5","product":"nginx"}],"deliberatelyUnmapped":["nginx"]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadMapFrom([]byte(tc.json)); err == nil {
				t.Errorf("LoadMapFrom(%s) succeeded, want error", tc.name)
			}
		})
	}
}

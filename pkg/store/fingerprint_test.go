package store

import "testing"

func attr(host, address, provider, region, jurisdiction string) AssetAttribution {
	return AssetAttribution{
		Host:         host,
		Address:      address,
		Provider:     provider,
		Region:       region,
		Jurisdiction: jurisdiction,
	}
}

// TestResolverOrderIsNotAChange is the point of sorting. Resolvers rotate
// their answers as a matter of course, and a fingerprint that preserved that
// order would report a regression on every other run — after which the list
// is noise and the real move is missed with it.
func TestResolverOrderIsNotAChange(t *testing.T) {
	first := HostingFingerprint([]AssetAttribution{
		attr("www.example.com", "203.0.113.1", "aws", "eu-west-2", "GB"),
		attr("www.example.com", "203.0.113.2", "aws", "eu-west-2", "GB"),
	})
	second := HostingFingerprint([]AssetAttribution{
		attr("www.example.com", "203.0.113.2", "aws", "eu-west-2", "GB"),
		attr("www.example.com", "203.0.113.1", "aws", "eu-west-2", "GB"),
	})

	if first != second {
		t.Fatalf("the same hosting in a different resolution order must fingerprint identically:\n  %q\n  %q", first, second)
	}
}

// TestAMoveBetweenRegionsIsAChange keeps a same-provider move visible. It does
// not cross a jurisdiction, but it is still a move, and an operator with a
// data-residency obligation is entitled to see it.
func TestAMoveBetweenRegionsIsAChange(t *testing.T) {
	before := HostingFingerprint([]AssetAttribution{attr("www.example.com", "203.0.113.1", "aws", "eu-west-2", "GB")})
	after := HostingFingerprint([]AssetAttribution{attr("www.example.com", "203.0.113.1", "aws", "us-east-1", "US")})

	if before == after {
		t.Fatal("a host moving between regions must be visible as a change")
	}
}

// TestNothingAttributedIsNotTheSameAsNoRecord keeps the two absences apart. An
// empty fingerprint would be indistinguishable from a missing snapshot, and
// only one of those says anything about the estate.
func TestNothingAttributedIsNotTheSameAsNoRecord(t *testing.T) {
	if HostingFingerprint(nil) == "" {
		t.Fatal("an attribution that established nothing must still render a value, so it is not mistaken for the absence of a record")
	}
}

// TestAnUnattributedAddressIsNamed stops an empty column inviting an
// assumption. An address that matched no published range says so.
func TestAnUnattributedAddressIsNamed(t *testing.T) {
	got := HostingFingerprint([]AssetAttribution{attr("www.example.com", "192.0.2.5", "", "", "")})

	if got == "" {
		t.Fatal("an unattributed address must appear in the fingerprint")
	}
	if want := "www.example.com|192.0.2.5|unattributed||"; got != want {
		t.Fatalf("an unattributed address must be named as such\n got %q\nwant %q", got, want)
	}
}

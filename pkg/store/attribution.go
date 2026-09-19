package store

import "time"

// AssetAttribution is one address an asset's name resolves to, and what that
// address could be attributed to.
//
// It is stored per address rather than per asset because a single name
// routinely resolves to several, and they need not agree: a name balanced
// across two providers, or across two jurisdictions, is a fact an operator
// needs to see rather than a discrepancy to be reduced to one winner.
//
// An empty Provider means the address matched no published range. That is
// not the same as "not hosted by a provider" — it is the absence of a match,
// and it is only trustworthy when the assessment's coverage says every range
// actually loaded. The adapter degrades the check to check_failed when they
// did not, so a reader who consults coverage alongside these rows can tell
// the two apart.
type AssetAttribution struct {
	AssetID string `json:"assetId"`

	// Host is the name that resolved, which may be the asset's own value or
	// a name within it, such as a mail exchanger or a nameserver.
	Host string `json:"host"`

	// Role says why the name was assessed — apex, host, mail exchanger,
	// nameserver — so that exposure on web infrastructure is distinguishable
	// from exposure on mail infrastructure.
	Role string `json:"role,omitempty"`

	// Address is the address the name resolved to.
	Address string `json:"address"`

	// Provider is the operator announcing the address. Empty means no
	// published range matched.
	Provider string `json:"provider,omitempty"`

	// Region is the operator's own region identifier, when published.
	Region string `json:"region,omitempty"`

	// Jurisdiction is the ISO 3166-1 alpha-2 country the region sits in.
	// Empty means unknown, never "assume home": an unknown jurisdiction
	// presented as the domestic one would turn a data-residency question
	// into a false reassurance.
	Jurisdiction string `json:"jurisdiction,omitempty"`

	// Source cites where the range data came from, so an attribution can be
	// checked rather than taken on trust.
	Source string `json:"source,omitempty"`

	LibraryVersion string    `json:"libraryVersion,omitempty"`
	ObservedAt     time.Time `json:"observedAt"`
}

// Attributed reports whether a published range matched this address.
//
// It exists so that callers ask the question in one place rather than
// comparing Provider against the empty string at each call site, where the
// meaning of the empty string is easy to forget.
func (a AssetAttribution) Attributed() bool { return a.Provider != "" }

// AttributionProvenance records where a provider's range data came from and
// when it was obtained.
//
// It is stored alongside the attributions rather than derived from them
// because it answers a different question: not "where is this address" but
// "on what basis do we say so, and how old is that basis". Comparing two
// runs without it cannot distinguish a host that moved from range data that
// was refreshed — the former is a change in the estate and the latter is not.
type AttributionProvenance struct {
	AssetID string `json:"assetId"`

	// Provider is the operator whose ranges these are.
	Provider string `json:"provider"`

	// URL is the endpoint the data came from, which may be a fallback rather
	// than the preferred one. A fallback is a different basis, not the same
	// data by another route, so the URL is compared and not just the
	// provider.
	URL string `json:"url"`

	// FetchedAt is when the data was obtained, not when it was used.
	FetchedAt time.Time `json:"fetchedAt"`
}

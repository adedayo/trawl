package vantage

import "runtime/debug"

// vantageModulePath is the module whose version stamps every observation.
const vantageModulePath = "github.com/adedayo/vantage"

// versionDevel marks observations produced by an unreleased working copy,
// which is what a local replace directive compiles in.
//
// It is not the same claim as "unknown". Unknown means the provenance could
// not be determined; this says precisely what the provenance is — code that
// was never published, and so cannot be fetched and compared against later.
// An observation carrying it should be treated as unreproducible rather than
// as merely undocumented.
const versionDevel = "devel"

// libraryVersion reports the version of the vantage module linked into this
// binary.
//
// The version is read from the build's own dependency list rather than being
// declared in code. A constant would be a second place to remember on every
// upgrade, and a stale one would misattribute observations to a library
// release that never produced them — which is worse than not knowing, because
// it looks like knowledge.
//
// It is not vres.Tool.Version. That field carries whatever the *embedder*
// stamps via audit.WithVersion, by vantage's own definition, so it answers
// "which build of Trawl ran this" and not "which build of the library decided
// what these findings mean". Only the latter tells a reader whether an
// observation can still be interpreted the way it was recorded.
//
// It returns an empty string when the build carries no information — a test
// binary, or a build stripped of its metadata. The caller decides what to put
// in its place; inventing a version here would be the misattribution this
// function exists to avoid.
func libraryVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path != vantageModulePath {
			continue
		}
		// A replaced module is the one actually compiled in, so the require
		// directive's version is not the answer: reporting it would attribute
		// observations to a release that did not produce them, which is the
		// specific error this function exists to avoid.
		if dep.Replace != nil {
			if dep.Replace.Version == "" {
				// A filesystem replace carries no version at all. Saying so
				// is more useful than saying nothing, because it tells a
				// reader the observation came from unpublished code.
				return versionDevel
			}
			return dep.Replace.Version
		}
		return dep.Version
	}
	return ""
}

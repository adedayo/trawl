package vantage

import "testing"

func TestVersionProbe(t *testing.T) {
	t.Logf("libraryVersion() = %q", libraryVersion())
}

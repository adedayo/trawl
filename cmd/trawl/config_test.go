package main

import "testing"

// TestScanModeDefaultsToInlineOnManagedPlatforms is the test that protects
// against silent data loss.
//
// On an autoscaling platform the instance is throttled once it has responded,
// so a scan detached onto a goroutine stalls rather than running. The caller
// holds a 202 for work that never happened. Defaulting to inline is what makes
// the response mean what it says.
func TestScanModeDefaultsToInlineOnManagedPlatforms(t *testing.T) {
	t.Setenv("K_SERVICE", "trawl")
	t.Setenv("TRAWL_SCAN_MODE", "")

	cfg := loadRuntimeConfig()

	if !cfg.managed {
		t.Fatal("K_SERVICE must be detected as an autoscaling platform")
	}
	if cfg.scanMode != scanInline {
		t.Fatalf("scanMode = %q, want %q on a managed platform", cfg.scanMode, scanInline)
	}
}

// TestScanModeDefaultsToBackgroundOnLongLivedContainers checks the converse:
// a Compose deployment keeps its asynchronous behaviour, where detaching is
// both safe and preferable.
func TestScanModeDefaultsToBackgroundOnLongLivedContainers(t *testing.T) {
	t.Setenv("K_SERVICE", "")
	t.Setenv("TRAWL_SCAN_MODE", "")

	cfg := loadRuntimeConfig()

	if cfg.managed {
		t.Fatal("a plain container must not be detected as an autoscaling platform")
	}
	if cfg.scanMode != scanBackground {
		t.Fatalf("scanMode = %q, want %q", cfg.scanMode, scanBackground)
	}
}

// TestScanModeHonoursExplicitOverride allows the documented escape hatch:
// Cloud Run with CPU always allocated can safely detach.
func TestScanModeHonoursExplicitOverride(t *testing.T) {
	t.Setenv("K_SERVICE", "trawl")
	t.Setenv("TRAWL_SCAN_MODE", "background")

	if got := loadRuntimeConfig().scanMode; got != scanBackground {
		t.Fatalf("scanMode = %q, want an explicit override to win", got)
	}
}

// TestPortTakesPrecedence guards the injected port.
//
// An instance that listens on the wrong port never passes its health check,
// and the deployment fails in a way that looks like the application being
// broken rather than misconfigured.
func TestPortTakesPrecedence(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("TRAWL_LISTEN_ADDR", ":8080")

	if got := loadRuntimeConfig().addr; got != ":9090" {
		t.Fatalf("addr = %q, want the injected PORT to win", got)
	}
}

func TestListenAddrUsedWhenNoPortInjected(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("TRAWL_LISTEN_ADDR", ":7000")

	if got := loadRuntimeConfig().addr; got != ":7000" {
		t.Fatalf("addr = %q, want :7000", got)
	}
}

func TestListenAddrFallsBackToDefault(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("TRAWL_LISTEN_ADDR", "")

	if got := loadRuntimeConfig().addr; got != ":8080" {
		t.Fatalf("addr = %q, want :8080", got)
	}
}

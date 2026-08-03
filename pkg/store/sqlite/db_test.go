package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestSQLiteStore_AssetLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_trawl.db")

	s, err := sqlite.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	asset := &store.Asset{
		ID:              "asset-1",
		Type:            store.AssetTypeSubdomain,
		Value:           "api.example.com",
		Status:          store.AssetStatusActive,
		DiscoverySource: "subfinder",
		Confidence:      0.95,
		FirstSeen:       time.Now(),
		LastSeen:        time.Now(),
	}

	if err := s.SaveAsset(ctx, asset); err != nil {
		t.Fatalf("Failed to save asset: %v", err)
	}

	fetched, err := s.GetAssetByID(ctx, "asset-1")
	if err != nil || fetched == nil {
		t.Fatalf("Failed to fetch asset by ID: %v", err)
	}
	if fetched.Value != "api.example.com" {
		t.Errorf("Expected value 'api.example.com', got '%s'", fetched.Value)
	}

	assets, err := s.GetAssets(ctx, store.AssetStatusActive)
	if err != nil || len(assets) != 1 {
		t.Fatalf("Expected 1 active asset, got %d", len(assets))
	}
}

func TestSQLiteStore_PostureRegressionConfirmation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_regression.db")

	s, err := sqlite.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Save parent asset to satisfy foreign key constraint
	asset := &store.Asset{
		ID:              "asset-1",
		Type:            store.AssetTypeSubdomain,
		Value:           "app.example.com",
		Status:          store.AssetStatusActive,
		DiscoverySource: "subfinder",
		Confidence:      0.9,
		FirstSeen:       time.Now(),
		LastSeen:        time.Now(),
	}
	if err := s.SaveAsset(ctx, asset); err != nil {
		t.Fatalf("Failed to save asset: %v", err)
	}

	// Observation 1: Good TLS
	reg1, err := s.RecordPostureObservation(ctx, "asset-1", "tls_version", "TLSv1.3")
	if err != nil {
		t.Fatalf("Failed 1st observation: %v", err)
	}
	if reg1 != nil {
		t.Errorf("Expected no regression on 1st observation")
	}

	// Observation 2: Downgraded TLS (differs from baseline -> triggers 2-consecutive failure confirmation)
	reg2, err := s.RecordPostureObservation(ctx, "asset-1", "tls_version", "TLSv1.0")
	if err != nil {
		t.Fatalf("Failed 2nd observation: %v", err)
	}
	if reg2 == nil {
		t.Fatalf("Expected confirmed regression on 2nd observation, got nil")
	}

	if reg2.PreviousValue != "TLSv1.3" || reg2.CurrentValue != "TLSv1.0" {
		t.Errorf("Unexpected regression values: prev=%s, curr=%s", reg2.PreviousValue, reg2.CurrentValue)
	}
}

package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/adedayo/trawl/pkg/store"
)

// ReplaceAssetAttribution swaps an asset's attribution in one transaction.
//
// Delete-then-insert rather than upsert, because these rows are a snapshot of
// a single assessment and not a history. An address the estate has stopped
// using would otherwise remain alongside one it currently uses, with nothing
// in the row to tell a reader which is current — and an operator reading a
// provider list would be looking at the union of every run ever performed.
//
// The whole replacement is one transaction so that a failure part way through
// leaves the previous snapshot intact. A half-written attribution is worse
// than a stale one: it is a partial view of the estate presented as a whole
// one.
func (s *SQLiteStore) ReplaceAssetAttribution(
	ctx context.Context,
	assetID string,
	rows []store.AssetAttribution,
	provenance []store.AttributionProvenance,
) error {
	if assetID == "" {
		return fmt.Errorf("attribution: an asset id is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("attribution: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM asset_attribution WHERE asset_id = ?`, assetID); err != nil {
		return fmt.Errorf("attribution: clearing previous rows: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM attribution_provenance WHERE asset_id = ?`, assetID); err != nil {
		return fmt.Errorf("attribution: clearing previous provenance: %w", err)
	}

	for _, r := range rows {
		// The primary key is (asset_id, host, address), and a host can
		// legitimately be reported twice for the same address across roles.
		// The later row wins rather than the insert failing: rejecting the
		// write would lose the entire snapshot over a duplicate that says the
		// same thing.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO asset_attribution
				(asset_id, host, role, address, provider, region, jurisdiction, source, library_version, observed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(asset_id, host, address) DO UPDATE SET
				role = excluded.role,
				provider = excluded.provider,
				region = excluded.region,
				jurisdiction = excluded.jurisdiction,
				source = excluded.source,
				library_version = excluded.library_version,
				observed_at = excluded.observed_at
		`, assetID, r.Host, r.Role, r.Address, r.Provider, r.Region, r.Jurisdiction, r.Source, r.LibraryVersion, r.ObservedAt); err != nil {
			return fmt.Errorf("attribution: saving %s/%s: %w", r.Host, r.Address, err)
		}
	}

	for _, p := range provenance {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO attribution_provenance (asset_id, provider, url, fetched_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(asset_id, provider, url) DO UPDATE SET
				fetched_at = excluded.fetched_at
		`, assetID, p.Provider, p.URL, p.FetchedAt); err != nil {
			return fmt.Errorf("attribution: saving provenance for %s: %w", p.Provider, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("attribution: commit: %w", err)
	}
	return nil
}

// GetAssetAttribution returns an asset's attribution. An empty assetID means
// every asset, following the coverage convention, so a portfolio view reads
// once rather than once per domain.
//
// Unattributed addresses sort last. They are the rows a reader should treat
// with most suspicion — an empty provider may mean the address is genuinely
// outside every published range, or that the ranges never loaded — and
// leading with them would bury the attributions that are actually settled.
func (s *SQLiteStore) GetAssetAttribution(ctx context.Context, assetID string) ([]store.AssetAttribution, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT asset_id, host, role, address, provider, region, jurisdiction, source, library_version, observed_at
		FROM asset_attribution
		WHERE ? = '' OR asset_id = ?
		ORDER BY asset_id ASC, (provider = '') ASC, provider ASC, host ASC, address ASC
	`, assetID, assetID)
	if err != nil {
		return nil, fmt.Errorf("attribution: querying: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanAttribution(rows)
}

func scanAttribution(rows *sql.Rows) ([]store.AssetAttribution, error) {
	// Never nil. An empty slice says "attribution ran and matched nothing";
	// a nil that a caller renders as "no data" says something weaker, and the
	// JSON encoding of the two differs.
	out := []store.AssetAttribution{}
	for rows.Next() {
		var a store.AssetAttribution
		if err := rows.Scan(
			&a.AssetID, &a.Host, &a.Role, &a.Address,
			&a.Provider, &a.Region, &a.Jurisdiction, &a.Source,
			&a.LibraryVersion, &a.ObservedAt,
		); err != nil {
			return nil, fmt.Errorf("attribution: scanning: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attribution: reading rows: %w", err)
	}
	return out, nil
}

// GetAttributionProvenance returns where each provider's ranges came from. An
// empty assetID means every asset, as for GetAssetAttribution.
//
// Oldest first, so a reader scanning the list meets the weakest basis before
// the strongest. A list led by a fetch from this morning invites the reader to
// stop there and conclude the attribution is fresh.
func (s *SQLiteStore) GetAttributionProvenance(ctx context.Context, assetID string) ([]store.AttributionProvenance, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT asset_id, provider, url, fetched_at
		FROM attribution_provenance
		WHERE ? = '' OR asset_id = ?
		ORDER BY fetched_at ASC, provider ASC
	`, assetID, assetID)
	if err != nil {
		return nil, fmt.Errorf("attribution: querying provenance: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []store.AttributionProvenance{}
	for rows.Next() {
		var p store.AttributionProvenance
		if err := rows.Scan(&p.AssetID, &p.Provider, &p.URL, &p.FetchedAt); err != nil {
			return nil, fmt.Errorf("attribution: scanning provenance: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attribution: reading provenance rows: %w", err)
	}
	return out, nil
}

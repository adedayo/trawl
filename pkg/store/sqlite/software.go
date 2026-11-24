package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adedayo/trawl/pkg/store"
)

// SaveAssetSoftware upserts a structured software observation for an asset.
//
// When a current record already exists on the same asset for the same product
// (matched by vendor+product when mapped, else by observed_name) with a
// different version, the earlier record is marked superseded with superseded_at
// set, and both remain readable.
func (s *SQLiteStore) SaveAssetSoftware(ctx context.Context, sw *store.AssetSoftware) error {
	now := s.now()
	if sw.FirstSeen.IsZero() {
		sw.FirstSeen = now
	}
	if sw.LastSeen.IsZero() {
		sw.LastSeen = now
	}
	if sw.Status == "" {
		sw.Status = store.SoftwareCurrent
	}
	if sw.VersionState == "" {
		if sw.Version != "" {
			sw.VersionState = store.SoftwareVersionKnown
		} else {
			sw.VersionState = store.SoftwareVersionUnknown
		}
	}
	if sw.MappingState == "" {
		if sw.Vendor != "" && sw.Product != "" {
			sw.MappingState = store.SoftwareMapped
		} else {
			sw.MappingState = store.SoftwareUnmapped
		}
	}
	if sw.ID == "" {
		sw.ID = store.AssetSoftwareID(sw.AssetID, strings.ToLower(strings.TrimSpace(sw.ObservedName)), sw.Version, sw.Source)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin software save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if sw.Status == store.SoftwareCurrent {
		if sw.MappingState == store.SoftwareMapped && sw.Vendor != "" && sw.Product != "" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE asset_software
				SET status = ?, superseded_at = ?
				WHERE asset_id = ? AND vendor = ? AND product = ? AND status = ? AND id != ?`,
				string(store.SoftwareSuperseded), formatTS(now),
				sw.AssetID, sw.Vendor, sw.Product, string(store.SoftwareCurrent), sw.ID,
			); err != nil {
				return fmt.Errorf("failed to supersede prior mapped software: %w", err)
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE asset_software
				SET status = ?, superseded_at = ?
				WHERE asset_id = ? AND LOWER(observed_name) = LOWER(?) AND status = ? AND id != ?`,
				string(store.SoftwareSuperseded), formatTS(now),
				sw.AssetID, sw.ObservedName, string(store.SoftwareCurrent), sw.ID,
			); err != nil {
				return fmt.Errorf("failed to supersede prior observed software: %w", err)
			}
		}
	}

	var supersededAt any
	if sw.SupersededAt != nil {
		supersededAt = formatTS(*sw.SupersededAt)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO asset_software (
			id, asset_id, observed_name, cpe_part, vendor, product,
			mapping_state, mapping_version, version, version_state,
			source, raw_string, status, first_seen, last_seen, superseded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			cpe_part = excluded.cpe_part,
			vendor = excluded.vendor,
			product = excluded.product,
			mapping_state = excluded.mapping_state,
			mapping_version = excluded.mapping_version,
			version_state = excluded.version_state,
			raw_string = excluded.raw_string,
			status = excluded.status,
			last_seen = excluded.last_seen,
			superseded_at = excluded.superseded_at
	`,
		sw.ID, sw.AssetID, sw.ObservedName, sw.CPEPart, sw.Vendor, sw.Product,
		string(sw.MappingState), sw.MappingVersion, sw.Version, string(sw.VersionState),
		sw.Source, sw.RawString, string(sw.Status), formatTS(sw.FirstSeen), formatTS(sw.LastSeen), supersededAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save asset software: %w", err)
	}

	return tx.Commit()
}

// GetAssetSoftware returns software records for one asset (or all assets when
// assetID is empty), ordered with current records first and most recently seen
// first.
func (s *SQLiteStore) GetAssetSoftware(ctx context.Context, assetID string) ([]store.AssetSoftware, error) {
	query := `
		SELECT id, asset_id, observed_name, cpe_part, vendor, product,
		       mapping_state, mapping_version, version, version_state,
		       source, raw_string, status, first_seen, last_seen, superseded_at
		FROM asset_software`
	var rows *sql.Rows
	var err error
	if assetID != "" {
		query += ` WHERE asset_id = ? ORDER BY CASE status WHEN 'current' THEN 0 ELSE 1 END, last_seen DESC, observed_name ASC`
		rows, err = s.reader().QueryContext(ctx, query, assetID)
	} else {
		query += ` ORDER BY asset_id ASC, CASE status WHEN 'current' THEN 0 ELSE 1 END, last_seen DESC, observed_name ASC`
		rows, err = s.reader().QueryContext(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query asset software: %w", err)
	}
	defer rows.Close()

	out := []store.AssetSoftware{}
	for rows.Next() {
		var sw store.AssetSoftware
		var firstSeen, lastSeen string
		var supersededAt sql.NullString
		if err := rows.Scan(
			&sw.ID, &sw.AssetID, &sw.ObservedName, &sw.CPEPart, &sw.Vendor, &sw.Product,
			&sw.MappingState, &sw.MappingVersion, &sw.Version, &sw.VersionState,
			&sw.Source, &sw.RawString, &sw.Status, &firstSeen, &lastSeen, &supersededAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan asset software: %w", err)
		}
		if sw.FirstSeen, err = parseTS("software first_seen", firstSeen); err != nil {
			return nil, err
		}
		if sw.LastSeen, err = parseTS("software last_seen", lastSeen); err != nil {
			return nil, err
		}
		if sw.SupersededAt, err = parseTSPtr("software superseded_at", supersededAt); err != nil {
			return nil, err
		}
		out = append(out, sw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read asset software: %w", err)
	}
	return out, nil
}

// ComputeSoftwareCoverage summarises what is known about an asset's software.
func (s *SQLiteStore) ComputeSoftwareCoverage(ctx context.Context, assetID string) (store.SoftwareCoverage, error) {
	items, err := s.GetAssetSoftware(ctx, assetID)
	if err != nil {
		return store.SoftwareCoverage{}, err
	}
	cov := store.SoftwareCoverage{
		AssetID: assetID,
		State:   store.SoftwareCoverageNotAssessed,
	}
	for _, sw := range items {
		if sw.Status == store.SoftwareSuperseded {
			cov.Superseded++
			continue
		}
		switch {
		case sw.MappingState == store.SoftwareUnmapped:
			cov.Unmapped++
		case sw.VersionState == store.SoftwareVersionKnown && sw.Version != "":
			cov.Identified++
		default:
			cov.VersionUnknown++
		}
	}
	switch {
	case cov.Identified > 0:
		cov.State = store.SoftwareCoverageIdentified
	case cov.VersionUnknown > 0:
		cov.State = store.SoftwareCoverageVersionUnknown
	case cov.Unmapped > 0:
		cov.State = store.SoftwareCoverageSeenUnmapped
	default:
		cov.State = store.SoftwareCoverageNotAssessed
		cov.Reason = s.softwareNotAssessedReason(ctx, assetID)
	}
	return cov, nil
}

// softwareNotAssessedReason explains why an asset currently has no software
// inventory records by inspecting the asset type and latest service probe
// observations. Never claims that absence of software data means zero risk.
func (s *SQLiteStore) softwareNotAssessedReason(ctx context.Context, assetID string) string {
	if asset, err := s.GetAssetByID(ctx, assetID); err == nil && asset != nil {
		if asset.Type == store.AssetTypeRepository {
			return "Repository asset; network service software probing does not apply."
		}
	}

	observations, err := s.GetServiceObservations(ctx, assetID)
	if err != nil || len(observations) == 0 {
		return "No service probe has run on this asset yet. Run a scan or service sweep to inspect exposed services."
	}

	// Keep the latest observation per (host, port, layer) so stale historical
	// probes do not distort the current explanation.
	type obsKey struct {
		host  string
		port  uint16
		layer string
	}
	latest := make(map[obsKey]store.ServiceObservation, len(observations))
	for _, o := range observations {
		k := obsKey{host: o.Host, port: o.Port, layer: o.Layer}
		if prev, ok := latest[k]; !ok || o.ObservedAt.After(prev.ObservedAt) {
			latest[k] = o
		}
	}

	var responding []string
	notResponding := 0
	inconclusive := 0
	var sampleErr string
	for _, o := range latest {
		switch {
		case o.Coverage.Assessed() && o.State == "responding":
			svc := o.Service
			if svc == "" {
				svc = o.Protocol
			}
			if svc == "" {
				svc = o.Layer
			}
			responding = append(responding, fmt.Sprintf("%s:%d", svc, o.Port))
		case o.Coverage.Assessed() && o.State == "not_responding":
			notResponding++
		default:
			inconclusive++
			if sampleErr == "" && o.Evidence != "" {
				var ev map[string]any
				if json.Unmarshal([]byte(o.Evidence), &ev) == nil {
					if e, _ := ev["error"].(string); strings.TrimSpace(e) != "" {
						sampleErr = strings.TrimSpace(e)
					}
				}
			}
		}
	}

	if len(responding) > 0 {
		if len(responding) > 4 {
			return fmt.Sprintf("%d responding service(s) (including %s) disclosed no software headers, cookies, HTML fingerprints, or banners.",
				len(responding), strings.Join(responding[:4], ", "))
		}
		return fmt.Sprintf("%d responding service(s) (%s) disclosed no software headers, cookies, HTML fingerprints, or banners.",
			len(responding), strings.Join(responding, ", "))
	}

	total := notResponding + inconclusive
	switch {
	case notResponding > 0 && inconclusive == 0:
		return fmt.Sprintf("Probed %d service(s); none responded (ports closed or refused).", total)
	case inconclusive > 0 && notResponding == 0:
		if sampleErr != "" {
			return fmt.Sprintf("Probed %d service(s); probes were inconclusive (%s).", total, sampleErr)
		}
		return fmt.Sprintf("Probed %d service(s); probes were inconclusive (timeout, firewall drop, or DNS failure).", total)
	default:
		return fmt.Sprintf("Probed %d service(s); %d did not respond and %d probe(s) were inconclusive.", total, notResponding, inconclusive)
	}
}

// GetSoftwareLookupCache retrieves a cached NVD lookup for a (part, vendor,
// product, version) key. Returns (nil, nil) if not cached.
func (s *SQLiteStore) GetSoftwareLookupCache(ctx context.Context, key string) (*store.SoftwareLookupCache, error) {
	var entry store.SoftwareLookupCache
	var queriedAt, cveJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, cpe_part, vendor, product, version, snapshot_id, fingerprint, queried_at, cve_json
		FROM software_lookup_cache
		WHERE id = ?
	`, key).Scan(
		&entry.ID, &entry.CPEPart, &entry.Vendor, &entry.Product, &entry.Version,
		&entry.SnapshotID, &entry.Fingerprint, &queriedAt, &cveJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query software lookup cache: %w", err)
	}
	if entry.QueriedAt, err = parseTS("lookup queried_at", queriedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(cveJSON), &entry.CVEs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal software lookup cache CVEs: %w", err)
	}
	return &entry, nil
}

// SaveSoftwareLookupCache persists a complete NVD lookup result for a
// (cpePart, vendor, product, version) tuple.
func (s *SQLiteStore) SaveSoftwareLookupCache(ctx context.Context, entry *store.SoftwareLookupCache) error {
	if entry.ID == "" {
		entry.ID = store.SoftwareLookupKey(entry.CPEPart, entry.Vendor, entry.Product, entry.Version)
	}
	if entry.QueriedAt.IsZero() {
		entry.QueriedAt = s.now()
	}
	cves := entry.CVEs
	if cves == nil {
		cves = []store.CandidateCVE{}
	}
	raw, err := json.Marshal(cves)
	if err != nil {
		return fmt.Errorf("failed to marshal lookup cache CVEs: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO software_lookup_cache (
			id, cpe_part, vendor, product, version, snapshot_id, fingerprint, queried_at, cve_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			snapshot_id = excluded.snapshot_id,
			fingerprint = excluded.fingerprint,
			queried_at = excluded.queried_at,
			cve_json = excluded.cve_json
	`,
		entry.ID, entry.CPEPart, entry.Vendor, entry.Product, entry.Version,
		entry.SnapshotID, entry.Fingerprint, formatTS(entry.QueriedAt), string(raw),
	)
	if err != nil {
		return fmt.Errorf("failed to save software lookup cache: %w", err)
	}
	return nil
}

// SaveCandidateFinding upserts an unconfirmed software-matched candidate and,
// in the same transaction, appends the corresponding finding_events row.
func (s *SQLiteStore) SaveCandidateFinding(ctx context.Context, cf *store.CandidateFinding) error {
	now := s.now()
	if cf.FirstSeen.IsZero() {
		cf.FirstSeen = now
	}
	if cf.LookupAt.IsZero() {
		cf.LookupAt = now
	}
	if cf.Status == "" {
		cf.Status = store.FindingOpen
	}
	if cf.SeveritySource == "" {
		cf.SeveritySource = "nvd"
	}
	if cf.ID == "" {
		cf.ID = store.CandidateFindingID(cf.AssetID, cf.Vendor, cf.Product, cf.Version, cf.CVE)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin candidate finding save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var prevStatus, prevSeverity, prevPriority string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(status,'open'), severity, priority FROM candidate_findings WHERE id = ?`, cf.ID).
		Scan(&prevStatus, &prevSeverity, &prevPriority)
	exists := err == nil
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to read candidate finding %s: %w", cf.ID, err)
	}

	var evType store.EventType
	var resolvedAt any
	switch {
	case !exists:
		evType = store.EventFirstSeen
	case prevStatus == string(store.FindingResolved) && cf.Status != store.FindingResolved:
		evType = store.EventReopened
		cf.Status = store.FindingReopened
	case prevStatus != string(store.FindingResolved) && cf.Status == store.FindingResolved:
		evType = store.EventResolved
	case prevSeverity != string(cf.Severity):
		evType = store.EventChanged
	case prevPriority != cf.Priority && prevPriority != "" && cf.Priority != "":
		evType = store.EventPriorityChanged
	}
	if exists && prevStatus == string(store.FindingReopened) && cf.Status == store.FindingOpen {
		cf.Status = store.FindingReopened
	}
	if cf.Status == store.FindingResolved {
		resolvedAt = formatTS(now)
	}

	kevInt := 0
	if cf.KEVListed {
		kevInt = 1
	}
	notReassessedInt := 0
	if cf.NotReassessed {
		notReassessedInt = 1
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO candidate_findings (
			id, asset_id, software_id, vendor, product, version, raw_string,
			cve, title, description, severity, severity_source, priority,
			epss, kev_listed, status, not_reassessed, not_reassessed_reason,
			lookup_snapshot_id, lookup_at, first_seen, last_seen, resolved_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			software_id = excluded.software_id,
			raw_string = excluded.raw_string,
			title = excluded.title,
			description = excluded.description,
			severity = excluded.severity,
			severity_source = excluded.severity_source,
			priority = excluded.priority,
			epss = excluded.epss,
			kev_listed = excluded.kev_listed,
			status = excluded.status,
			not_reassessed = excluded.not_reassessed,
			not_reassessed_reason = excluded.not_reassessed_reason,
			lookup_snapshot_id = excluded.lookup_snapshot_id,
			lookup_at = excluded.lookup_at,
			last_seen = excluded.last_seen,
			resolved_at = excluded.resolved_at
	`,
		cf.ID, cf.AssetID, cf.SoftwareID, cf.Vendor, cf.Product, cf.Version, cf.RawString,
		cf.CVE, cf.Title, cf.Description, string(cf.Severity), cf.SeveritySource, cf.Priority,
		cf.EPSS, kevInt, string(cf.Status), notReassessedInt, cf.NotReassessedReason,
		cf.LookupSnapshotID, formatTS(cf.LookupAt), formatTS(cf.FirstSeen), formatTS(now), resolvedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save candidate finding: %w", err)
	}

	for _, feedName := range []string{store.FeedKEV, store.FeedEPSS, store.FeedNVD} {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO candidate_enrichments (candidate_id, feed, cve, state)
			VALUES (?, ?, ?, ?)`,
			cf.ID, feedName, cf.CVE, string(store.EnrichmentNotChecked),
		); err != nil {
			return fmt.Errorf("failed to seed %s enrichment for candidate %s: %w", feedName, cf.ID, err)
		}
	}

	if evType != "" {
		ev := s.newEvent(ctx, store.SourceCandidate, cf.ID, cf.AssetID, evType)
		ev.PrevSeverity = store.FindingSeverity(prevSeverity)
		ev.Severity = cf.Severity
		ev.PrevPriority = prevPriority
		ev.Priority = cf.Priority
		if ev.SnapshotID == "" {
			ev.SnapshotID = cf.LookupSnapshotID
		}
		if err := appendEvent(ctx, tx, &ev); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetCandidateFindings returns candidate findings for one asset (or all assets
// when assetID is empty), including their feed enrichments.
func (s *SQLiteStore) GetCandidateFindings(ctx context.Context, assetID string) ([]store.CandidateFinding, error) {
	query := `
		SELECT id, asset_id, software_id, vendor, product, version, raw_string,
		       cve, title, COALESCE(description,''), severity, severity_source, priority,
		       epss, kev_listed, status, not_reassessed, not_reassessed_reason,
		       lookup_snapshot_id, lookup_at, first_seen, last_seen, resolved_at
		FROM candidate_findings`
	var rows *sql.Rows
	var err error
	if assetID != "" {
		query += ` WHERE asset_id = ? ORDER BY last_seen DESC, cve ASC`
		rows, err = s.reader().QueryContext(ctx, query, assetID)
	} else {
		query += ` ORDER BY last_seen DESC, cve ASC`
		rows, err = s.reader().QueryContext(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query candidate findings: %w", err)
	}
	defer rows.Close()

	out := []store.CandidateFinding{}
	for rows.Next() {
		var cf store.CandidateFinding
		var kevInt, notReassessedInt int
		var lookupAt, firstSeen, lastSeen string
		var resolvedAt sql.NullString
		if err := rows.Scan(
			&cf.ID, &cf.AssetID, &cf.SoftwareID, &cf.Vendor, &cf.Product, &cf.Version, &cf.RawString,
			&cf.CVE, &cf.Title, &cf.Description, &cf.Severity, &cf.SeveritySource, &cf.Priority,
			&cf.EPSS, &kevInt, &cf.Status, &notReassessedInt, &cf.NotReassessedReason,
			&cf.LookupSnapshotID, &lookupAt, &firstSeen, &lastSeen, &resolvedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan candidate finding: %w", err)
		}
		cf.KEVListed = kevInt == 1
		cf.NotReassessed = notReassessedInt == 1
		if cf.LookupAt, err = parseTS("candidate lookup_at", lookupAt); err != nil {
			return nil, err
		}
		if cf.FirstSeen, err = parseTS("candidate first_seen", firstSeen); err != nil {
			return nil, err
		}
		if cf.LastSeen, err = parseTS("candidate last_seen", lastSeen); err != nil {
			return nil, err
		}
		if cf.ResolvedAt, err = parseTSPtr("candidate resolved_at", resolvedAt); err != nil {
			return nil, err
		}
		out = append(out, cf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read candidate findings: %w", err)
	}
	rows.Close()

	for i := range out {
		out[i].Enrichments, err = s.GetCandidateEnrichments(ctx, out[i].ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query candidate enrichments: %w", err)
		}
	}
	return out, nil
}

// GetCandidateEnrichments returns feed enrichments for one candidate finding.
func (s *SQLiteStore) GetCandidateEnrichments(ctx context.Context, candidateID string) ([]store.FindingEnrichment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.candidate_id, e.feed, e.cve, e.state, COALESCE(e.snapshot_id,''),
		       e.epss, e.kev_listed, e.checked_at,
		       fs.id, fs.feed, fs.source_url, fs.retrieved_at, fs.content_digest, fs.record_count
		FROM candidate_enrichments e
		LEFT JOIN feed_snapshots fs ON e.snapshot_id = fs.id
		WHERE e.candidate_id = ?
		ORDER BY e.feed ASC
	`, candidateID)
	if err != nil {
		return nil, fmt.Errorf("failed to query candidate enrichments: %w", err)
	}
	defer rows.Close()

	out := []store.FindingEnrichment{}
	for rows.Next() {
		var e store.FindingEnrichment
		var epss sql.NullFloat64
		var kev sql.NullInt64
		var checkedAt sql.NullString
		var fsID, fsFeed, fsURL, fsRetrieved, fsDigest sql.NullString
		var fsCount sql.NullInt64
		if err := rows.Scan(
			&e.FindingID, &e.Feed, &e.CVE, &e.State, &e.SnapshotID,
			&epss, &kev, &checkedAt,
			&fsID, &fsFeed, &fsURL, &fsRetrieved, &fsDigest, &fsCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan candidate enrichment: %w", err)
		}
		if epss.Valid {
			v := epss.Float64
			e.EPSS = &v
		}
		if kev.Valid {
			b := kev.Int64 == 1
			e.KEVListed = &b
		}
		if e.CheckedAt, err = parseTSPtr("candidate enrichment checked_at", checkedAt); err != nil {
			return nil, err
		}
		if fsID.Valid && fsID.String != "" {
			ret, err := parseTS("feed snapshot retrieved_at", fsRetrieved.String)
			if err != nil {
				return nil, err
			}
			e.Snapshot = &store.FeedSnapshot{
				ID:            fsID.String,
				Feed:          fsFeed.String,
				SourceURL:     fsURL.String,
				RetrievedAt:   ret,
				ContentDigest: fsDigest.String,
				RecordCount:   int(fsCount.Int64),
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SaveCandidateEnrichment upserts one feed enrichment row for a candidate.
func (s *SQLiteStore) SaveCandidateEnrichment(ctx context.Context, enrichment *store.FindingEnrichment) error {
	var snapshotID any
	if enrichment.SnapshotID != "" {
		snapshotID = enrichment.SnapshotID
	}
	var epss any
	if enrichment.EPSS != nil {
		epss = *enrichment.EPSS
	}
	var kev any
	if enrichment.KEVListed != nil {
		if *enrichment.KEVListed {
			kev = 1
		} else {
			kev = 0
		}
	}
	var checkedAt any
	if enrichment.CheckedAt != nil {
		checkedAt = formatTS(*enrichment.CheckedAt)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO candidate_enrichments (candidate_id, feed, cve, state, snapshot_id, epss, kev_listed, checked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(candidate_id, feed) DO UPDATE SET
			cve = excluded.cve,
			state = excluded.state,
			snapshot_id = excluded.snapshot_id,
			epss = excluded.epss,
			kev_listed = excluded.kev_listed,
			checked_at = excluded.checked_at
	`, enrichment.FindingID, enrichment.Feed, enrichment.CVE, string(enrichment.State), snapshotID, epss, kev, checkedAt)
	if err != nil {
		return fmt.Errorf("failed to save candidate enrichment: %w", err)
	}
	return nil
}

// MarkAssetCandidatesNotReassessed marks open/reopened candidates on an asset
// as not re-assessed when a later scan reported no software data or a lookup
// failed, leaving them open rather than resolving them.
func (s *SQLiteStore) MarkAssetCandidatesNotReassessed(ctx context.Context, assetID, reason string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE candidate_findings
		SET not_reassessed = 1, not_reassessed_reason = ?
		WHERE asset_id = ? AND status != ?
	`, reason, assetID, string(store.FindingResolved))
	if err != nil {
		return fmt.Errorf("failed to mark asset candidates not reassessed: %w", err)
	}
	return nil
}

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) GetRegressions(ctx context.Context) ([]store.Regression, error) {
	query := `SELECT id, asset_id, attribute_type, previous_value, current_value, consecutive_fails, confirmed_at FROM regressions ORDER BY confirmed_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query regressions: %w", err)
	}
	defer rows.Close()

	regressions := []store.Regression{}
	for rows.Next() {
		var r store.Regression
		var confirmedAt string
		if err := rows.Scan(&r.ID, &r.AssetID, &r.AttributeType, &r.PreviousValue, &r.CurrentValue, &r.ConsecutiveFails, &confirmedAt); err != nil {
			return nil, fmt.Errorf("failed to scan regression: %w", err)
		}
		r.ConfirmedAt, _ = time.Parse(time.RFC3339, confirmedAt)
		regressions = append(regressions, r)
	}
	return regressions, nil
}

// RecordPostureBaseline records an observation without letting it raise a
// regression.
//
// It exists for changes whose cause is known not to be the estate. Provider
// range data is republished constantly, and a prefix moving between two
// publications changes what an address attributes to without anything in the
// estate having moved at all. Reporting that as a degradation would fill the
// regression list with noise, and a list of mostly-noise is one an operator
// stops reading — at which point the real degradation goes unseen too.
//
// The snapshot is still written, deliberately. Skipping it entirely would
// leave the baseline sitting at the pre-refresh value, so the next comparison
// would resurface the same change and raise it as a regression a run later,
// which is the suppression failing quietly rather than working.
func (s *SQLiteStore) RecordPostureBaseline(ctx context.Context, assetID string, attributeType string, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO posture_snapshots (asset_id, attribute_type, value, observed_at) VALUES (?, ?, ?, ?)`,
		assetID, attributeType, value, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to insert posture baseline: %w", err)
	}
	return nil
}

func (s *SQLiteStore) RecordPostureObservation(ctx context.Context, assetID string, attributeType string, value string) (*store.Regression, error) {
	nowStr := time.Now().Format(time.RFC3339)

	// Insert posture snapshot
	_, err := s.db.ExecContext(ctx, `INSERT INTO posture_snapshots (asset_id, attribute_type, value, observed_at) VALUES (?, ?, ?, ?)`,
		assetID, attributeType, value, nowStr)
	if err != nil {
		return nil, fmt.Errorf("failed to insert posture snapshot: %w", err)
	}

	// Fetch last 2 snapshots
	query := `SELECT value FROM posture_snapshots WHERE asset_id = ? AND attribute_type = ? ORDER BY id DESC LIMIT 2`
	rows, err := s.db.QueryContext(ctx, query, assetID, attributeType)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent snapshots: %w", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err == nil {
			values = append(values, v)
		}
	}

	// If we have at least 2 consecutive observations and both differ from initial baseline
	if len(values) == 2 && values[0] != values[1] {
		regID := fmt.Sprintf("reg-%s-%s-%d", assetID, attributeType, time.Now().Unix())
		reg := &store.Regression{
			ID:               regID,
			AssetID:          assetID,
			AttributeType:    attributeType,
			PreviousValue:    values[1],
			CurrentValue:     values[0],
			ConsecutiveFails: 2,
			ConfirmedAt:      time.Now(),
		}

		insertReg := `
		INSERT INTO regressions (id, asset_id, attribute_type, previous_value, current_value, consecutive_fails, confirmed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		_, err := s.db.ExecContext(ctx, insertReg, reg.ID, reg.AssetID, reg.AttributeType, reg.PreviousValue, reg.CurrentValue, reg.ConsecutiveFails, nowStr)
		if err != nil {
			return nil, fmt.Errorf("failed to record confirmed regression: %w", err)
		}
		return reg, nil
	}

	return nil, nil
}

// emailControls is the four-state posture as persisted.
//
// It is held as JSON rather than as forty columns because it is nested, it is
// always read whole, and nothing queries across its parts. The boolean columns
// beside it are a lossy mirror kept only so that an older build can open the
// database; this is the record.
type emailControls struct {
	SPF    store.EmailControl `json:"spf"`
	DKIM   store.EmailControl `json:"dkim"`
	DMARC  store.EmailControl `json:"dmarc"`
	MTASTS store.EmailControl `json:"mtaSts"`
	TLSRPT store.EmailControl `json:"tlsRpt"`
	BIMI   store.EmailControl `json:"bimi"`
	CAA    store.EmailControl `json:"caa"`
}

// emailDetails is the parsed evidence severity is computed from.
type emailDetails struct {
	DMARCSubdomainPolicy  string   `json:"dmarcSubdomainPolicy,omitempty"`
	DMARCPercent          int      `json:"dmarcPercent"`
	DMARCAlignmentSPF     string   `json:"dmarcAlignmentSpf,omitempty"`
	DMARCAlignmentDKIM    string   `json:"dmarcAlignmentDkim,omitempty"`
	DMARCReporting        bool     `json:"dmarcReporting"`
	SPFAllMechanism       string   `json:"spfAllMechanism,omitempty"`
	SPFLookups            int      `json:"spfLookups"`
	DKIMSelectorsExamined []string `json:"dkimSelectorsExamined,omitempty"`
	DKIMSelectorsFound    []string `json:"dkimSelectorsFound,omitempty"`
}

func (s *SQLiteStore) GetEmailPostures(ctx context.Context) ([]store.EmailPosture, error) {
	query := `SELECT domain, dmarc_policy, priority, last_checked, controls, details FROM email_postures ORDER BY domain ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query email postures: %w", err)
	}
	defer rows.Close()

	postures := []store.EmailPosture{}
	for rows.Next() {
		var (
			p                         store.EmailPosture
			priority                  string
			lastChecked               string
			controlsJSON, detailsJSON sql.NullString
		)
		if err := rows.Scan(&p.Domain, &p.DMARCPolicy, &priority, &lastChecked,
			&controlsJSON, &detailsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan email posture: %w", err)
		}
		p.Priority = store.FindingSeverity(priority)
		p.LastChecked, _ = time.Parse(time.RFC3339, lastChecked)

		// A row written before the widening has no controls. Every state is
		// then left at its zero value, which is not a recognised state and so
		// reads as unassessed — the honest account, since what that row holds
		// cannot distinguish an absent control from a failed lookup. Inventing
		// "ok" from a legacy boolean would be the collapse this change exists
		// to undo, preserved in the migration.
		if controlsJSON.Valid && controlsJSON.String != "" {
			var c emailControls
			if err := json.Unmarshal([]byte(controlsJSON.String), &c); err != nil {
				return nil, fmt.Errorf("failed to read email controls for %s: %w", p.Domain, err)
			}
			p.SPF, p.DKIM, p.DMARC = c.SPF, c.DKIM, c.DMARC
			p.MTASTS, p.TLSRPT, p.BIMI, p.CAA = c.MTASTS, c.TLSRPT, c.BIMI, c.CAA
		}

		if detailsJSON.Valid && detailsJSON.String != "" {
			var d emailDetails
			if err := json.Unmarshal([]byte(detailsJSON.String), &d); err != nil {
				return nil, fmt.Errorf("failed to read email details for %s: %w", p.Domain, err)
			}
			p.DMARCSubdomainPolicy = d.DMARCSubdomainPolicy
			p.DMARCPercent = d.DMARCPercent
			p.DMARCAlignmentSPF = d.DMARCAlignmentSPF
			p.DMARCAlignmentDKIM = d.DMARCAlignmentDKIM
			p.DMARCReporting = d.DMARCReporting
			p.SPFAllMechanism = d.SPFAllMechanism
			p.SPFLookups = d.SPFLookups
			p.DKIMSelectorsExamined = d.DKIMSelectorsExamined
			p.DKIMSelectorsFound = d.DKIMSelectorsFound
		}

		postures = append(postures, p)
	}
	return postures, rows.Err()
}

func (s *SQLiteStore) SaveEmailPosture(ctx context.Context, ep *store.EmailPosture) error {
	query := `
	INSERT INTO email_postures (domain, spf_valid, dkim_found, dmarc_policy, priority, last_checked, controls, details)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(domain) DO UPDATE SET
		spf_valid = excluded.spf_valid,
		dkim_found = excluded.dkim_found,
		dmarc_policy = excluded.dmarc_policy,
		priority = excluded.priority,
		last_checked = excluded.last_checked,
		controls = excluded.controls,
		details = excluded.details
	`

	controls, err := json.Marshal(emailControls{
		SPF: ep.SPF, DKIM: ep.DKIM, DMARC: ep.DMARC,
		MTASTS: ep.MTASTS, TLSRPT: ep.TLSRPT, BIMI: ep.BIMI, CAA: ep.CAA,
	})
	if err != nil {
		return fmt.Errorf("failed to encode email controls: %w", err)
	}

	details, err := json.Marshal(emailDetails{
		DMARCSubdomainPolicy:  ep.DMARCSubdomainPolicy,
		DMARCPercent:          ep.DMARCPercent,
		DMARCAlignmentSPF:     ep.DMARCAlignmentSPF,
		DMARCAlignmentDKIM:    ep.DMARCAlignmentDKIM,
		DMARCReporting:        ep.DMARCReporting,
		SPFAllMechanism:       ep.SPFAllMechanism,
		SPFLookups:            ep.SPFLookups,
		DKIMSelectorsExamined: ep.DKIMSelectorsExamined,
		DKIMSelectorsFound:    ep.DKIMSelectorsFound,
	})
	if err != nil {
		return fmt.Errorf("failed to encode email details: %w", err)
	}

	// The legacy booleans are written only so an older build can still read
	// the row. Only an assessed, passing control sets one: an unassessed
	// control must not mirror as true, or downgrading would resurrect the
	// collapse in its most misleading form — an outage reading as a control
	// in place.
	spfInt, dkimInt := 0, 0
	if ep.SPF.State.Passing() {
		spfInt = 1
	}
	if ep.DKIM.State.Passing() {
		dkimInt = 1
	}

	if ep.LastChecked.IsZero() {
		ep.LastChecked = time.Now()
	}

	_, err = s.db.ExecContext(ctx, query,
		ep.Domain,
		spfInt,
		dkimInt,
		ep.DMARCPolicy,
		string(ep.Priority),
		ep.LastChecked.Format(time.RFC3339),
		string(controls),
		string(details),
	)
	if err != nil {
		return fmt.Errorf("failed to save email posture: %w", err)
	}
	return nil
}

package service

import (
	"context"
	"fmt"
	"github.com/adedayo/trawl/pkg/scanner"
	"github.com/adedayo/trawl/pkg/store"
	"strings"
)

// EmailScannerService provides a higher‑level API to run the email audit scanner
// and persist the results using the store layer.
type EmailScannerService struct {
	store store.Store
}

func NewEmailScannerService(s store.Store) *EmailScannerService {
	return &EmailScannerService{store: s}
}

// ScanAndSave runs the scanner for the given domain, maps the results to the
// store.EmailPosture model and saves it. It returns the persisted posture.
func (svc *EmailScannerService) ScanAndSave(ctx context.Context, domain string) (store.EmailPosture, error) {
	ep, err := scanner.ScanDomain(ctx, domain)
	if err != nil {
		return store.EmailPosture{}, fmt.Errorf("email scan failed: %w", err)
	}

	// Map fields from scanner.EmailPosture to store.EmailPosture
	// The store struct expects bool flags for MTA‑STS, DNSSEC and DANE.
	var mtaFound bool
	var mtaMode string
	if ep.MTASTS != "" && !strings.HasPrefix(strings.ToLower(ep.MTASTS), "error") && ep.MTASTS != "not found" {
		mtaFound = true
		mtaMode = ep.MTASTS // raw TXT value – callers can interpret mode if needed
	}

	dnssecValid := strings.EqualFold(ep.DNSSEC, "enabled")
	daneValid := ep.DANE != "" && ep.DANE != "not found" && !strings.HasPrefix(strings.ToLower(ep.DANE), "error")

	// Build store model. Other fields (SPFValid, DKIMFound, DMARCPolicy) are taken from scanner results.
	dmarcPolicy := ""
	// Extract policy value from DMARC TXT record (e.g., "p=reject")
	if strings.Contains(strings.ToLower(ep.DMARCResult), "p=") {
		parts := strings.Split(ep.DMARCResult, ";")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(strings.ToLower(p), "p=") {
				dmarcPolicy = strings.TrimPrefix(strings.ToLower(p), "p=")
				dmarcPolicy = strings.TrimSpace(dmarcPolicy)
				break
			}
		}
	}
	posture := store.EmailPosture{
		Domain:      ep.Domain,
		SPFValid:    ep.SPFResult != "" && !strings.HasPrefix(strings.ToLower(ep.SPFResult), "error"),
		DKIMFound:   ep.DKIMResult != "" && !strings.HasPrefix(strings.ToLower(ep.DKIMResult), "error"),
		DMARCPolicy: dmarcPolicy,
		// Priority is not part of scanner output – keep empty for now.
		Priority: "",
		// MTA‑STS fields
		MTAStsFound: mtaFound,
		MTAStsMode:  mtaMode,
		// DNSSEC/DANE flags
		DNSSECValid: dnssecValid,
		DANEValid:   daneValid,
	}

	if err := svc.store.SaveEmailPosture(ctx, &posture); err != nil {
		return store.EmailPosture{}, fmt.Errorf("failed to save email posture: %w", err)
	}
	return posture, nil
}

package vantage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	vobs "github.com/adedayo/vantage/pkg/observation"
	vprobe "github.com/adedayo/vantage/pkg/probe"

	"github.com/adedayo/trawl/pkg/store"
)

// ServiceRequest asks Vantage to probe one already discovered and authorised
// host. The profile controls candidate breadth; it does not discover hosts.
type ServiceRequest struct {
	AssetID string
	Host    string
	Profile vprobe.DiscoveryProfile
	Custom  []vprobe.Request
}

// ProbeServices runs the Vantage service probe and translates observations to
// Trawl storage types. It does not infer attacker contact or risk.
func ProbeServices(ctx context.Context, prober vprobe.Prober, request ServiceRequest) ([]store.ServiceObservation, error) {
	if prober == nil {
		return nil, fmt.Errorf("vantage: a service prober is required")
	}
	if request.AssetID == "" || request.Host == "" {
		return nil, fmt.Errorf("vantage: asset ID and host are required")
	}
	requests := request.Custom
	if len(requests) == 0 {
		profile := request.Profile
		if profile == "" {
			profile = vprobe.DiscoveryMostCommon
		}
		var err error
		requests, err = vprobe.RequestsForProfile(request.Host, profile)
		if err != nil {
			return nil, err
		}
	}
	observations := prober.ProbeMany(ctx, requests)
	if len(observations) != len(requests) {
		return nil, fmt.Errorf("vantage: prober returned %d observations for %d requests", len(observations), len(requests))
	}
	result := make([]store.ServiceObservation, 0, len(observations))
	for _, observation := range observations {
		evidence, err := json.Marshal(observation.Evidence)
		if err != nil {
			return nil, fmt.Errorf("vantage: encoding service evidence: %w", err)
		}
		coverage := store.CoverageCheckFailed
		if observation.State == vobs.ServiceResponding || observation.State == vobs.ServiceNotResponding {
			coverage = store.CoverageOK
		}
		translated := store.ServiceObservation{
			AssetID: request.AssetID, Host: observation.Host, Port: observation.Port,
			Service: observation.Service, Transport: observation.Transport,
			Protocol: observation.Protocol, Layer: string(observation.Layer),
			State: string(observation.State), Coverage: coverage,
			Evidence: string(evidence), Profile: observation.ProbeProfile,
			ObservedAt: observation.ObservedAt,
		}
		result = append(result, translated)
	}
	return result, nil
}

// ServiceAdvisories turns one probe batch into deterministic, actionable
// findings. It deliberately uses only observed evidence; it never infers
// attacker contact or risk probability.
func ServiceAdvisories(ctx context.Context, save func(*store.Finding) error, observations []store.ServiceObservation, now time.Time) error {
	for _, observation := range observations {
		if observation.State != "responding" || !observation.Coverage.Assessed() {
			continue
		}
		evidence := map[string]any{}
		if err := json.Unmarshal([]byte(observation.Evidence), &evidence); err != nil {
			return fmt.Errorf("vantage: decoding service evidence: %w", err)
		}
		service := observation.Service
		if service == "" {
			service = observation.Protocol
		}
		severity := serviceSeverity(service, observation.Port)
		proof := fmt.Sprintf("host=%s port=%d service=%s layer=%s evidence=%s",
			observation.Host, observation.Port, service, observation.Layer, observation.Evidence)
		if err := save(&store.Finding{
			ID: findingID(observation, "exposed"), AssetID: observation.AssetID,
			Title:       fmt.Sprintf("Externally reachable %s service", service),
			Description: "A service responded from the authorised external vantage point. Confirm that this exposure is intentional and restrict it if it is not required.",
			Severity:    severity, Priority: string(severity), Category: "service-exposure", Proof: proof,
			FirstSeen: observation.ObservedAt, LastSeen: observation.ObservedAt,
		}); err != nil {
			return err
		}

		if strings.EqualFold(observation.Protocol, "https") && strings.TrimSpace(stringValue(evidence["hsts"])) == "" {
			if err := saveAdvisory(save, observation, "missing-hsts", store.SeverityLow,
				"HTTPS service does not advertise HSTS", "Add a Strict-Transport-Security header after confirming all subresources and subdomains support HTTPS", now); err != nil {
				return err
			}
		}
		if tlsVersion := stringValue(evidence["tls_version"]); tlsVersion == "TLS 1.0" || tlsVersion == "TLS 1.1" {
			if err := saveAdvisory(save, observation, "weak-tls-version", store.SeverityHigh,
				"HTTPS service negotiates an obsolete TLS version", "Disable TLS 1.0 and TLS 1.1 and require TLS 1.2 or newer", now); err != nil {
				return err
			}
		}
		if strings.EqualFold(observation.Protocol, "http") && observation.Port == 80 &&
			observationEvidenceStatus(evidence) >= 200 && observationEvidenceStatus(evidence) < 400 &&
			strings.TrimSpace(stringValue(evidence["location"])) == "" {
			if err := saveAdvisory(save, observation, "http-no-https-redirect", store.SeverityLow,
				"HTTP service does not redirect to HTTPS", "Redirect HTTP to HTTPS and verify the HTTPS endpoint is the canonical service", now); err != nil {
				return err
			}
		}
		if expiry := stringValue(evidence["certificate_expiry"]); expiry != "" {
			certificateExpiry, err := time.Parse(time.RFC3339, expiry)
			if err == nil {
				if certificateExpiry.Before(now) {
					if err := saveAdvisory(save, observation, "expired-certificate", store.SeverityHigh,
						"HTTPS service presents an expired certificate", "Renew the certificate and verify automated renewal before the next scan", now); err != nil {
						return err
					}
				} else if certificateExpiry.Before(now.Add(30 * 24 * time.Hour)) {
					if err := saveAdvisory(save, observation, "expiring-certificate", store.SeverityMedium,
						"HTTPS service certificate expires within 30 days", "Renew the certificate and verify automated renewal", now); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func observationEvidenceStatus(evidence map[string]any) int {
	value, ok := evidence["http_status"].(float64)
	if !ok {
		return 0
	}
	return int(value)
}

// ActiveServiceFindingIDs returns the deterministic IDs that should remain
// open after the current probe batch. Anything previously generated by the
// service workflow but absent here can safely be resolved by the caller.
func ActiveServiceFindingIDs(observations []store.ServiceObservation, now time.Time) map[string]bool {
	active := map[string]bool{}
	for _, observation := range observations {
		if observation.State != "responding" || !observation.Coverage.Assessed() {
			continue
		}
		active[findingID(observation, "exposed")] = true
		var evidence map[string]any
		if json.Unmarshal([]byte(observation.Evidence), &evidence) != nil {
			continue
		}
		if strings.EqualFold(observation.Protocol, "https") {
			if strings.TrimSpace(stringValue(evidence["hsts"])) == "" {
				active[findingID(observation, "missing-hsts")] = true
			}
			if expiry := stringValue(evidence["certificate_expiry"]); expiry != "" {
				if certificateExpiry, err := time.Parse(time.RFC3339, expiry); err == nil {
					if certificateExpiry.Before(now) {
						active[findingID(observation, "expired-certificate")] = true
					} else if certificateExpiry.Before(now.Add(30 * 24 * time.Hour)) {
						active[findingID(observation, "expiring-certificate")] = true
					}
				}
			}
			if tlsVersion := stringValue(evidence["tls_version"]); tlsVersion == "TLS 1.0" || tlsVersion == "TLS 1.1" {
				active[findingID(observation, "weak-tls-version")] = true
			}
		}
		if strings.EqualFold(observation.Protocol, "http") && observation.Port == 80 &&
			observationEvidenceStatus(evidence) >= 200 && observationEvidenceStatus(evidence) < 400 &&
			strings.TrimSpace(stringValue(evidence["location"])) == "" {
			active[findingID(observation, "http-no-https-redirect")] = true
		}
	}
	return active
}

func saveAdvisory(save func(*store.Finding) error, observation store.ServiceObservation, code string, severity store.FindingSeverity, title, remediation string, now time.Time) error {
	return save(&store.Finding{
		ID: findingID(observation, code), AssetID: observation.AssetID, Title: title,
		Description: remediation, Severity: severity, Priority: string(severity),
		Category: "service-posture", Proof: observation.Evidence,
		FirstSeen: observation.ObservedAt, LastSeen: now,
	})
}

func findingID(observation store.ServiceObservation, code string) string {
	return fmt.Sprintf("service:%s:%s:%d:%s:%s", observation.AssetID, observation.Host, observation.Port, observation.Layer, code)
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func serviceSeverity(service string, port uint16) store.FindingSeverity {
	switch strings.ToLower(service) {
	case "docker", "docker-tls", "redis", "mongodb", "mysql", "postgresql", "mssql", "oracle", "elasticsearch", "kubernetes-api":
		return store.SeverityHigh
	case "ssh", "telnet", "smb", "netbios-ssn", "rdp", "ftp", "smtp", "submission", "imap", "imaps", "pop3", "pop3s", "smtps":
		return store.SeverityMedium
	default:
		if port == 80 || port == 443 {
			return store.SeverityLow
		}
		return store.SeverityMedium
	}
}

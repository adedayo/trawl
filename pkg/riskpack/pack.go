// Package riskpack loads the versioned, signed parameters used by Trawl's
// probability models and computes measured-state precision gain.
package riskpack

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

const schemaVersion = 1

type VerificationStatus string
type CalibrationLabel string

const (
	NeedsVerification VerificationStatus = "needs-verification"
	Verified          VerificationStatus = "verified"
	LocallyOverridden VerificationStatus = "locally-overridden"

	Illustrative CalibrationLabel = "illustrative"
	Sourced      CalibrationLabel = "sourced"
	Calibrated   CalibrationLabel = "calibrated"
)

// Parameter is the only representation allowed for a model input. Keeping the
// value as JSON lets one pack carry probabilities, rates, and other scalar
// types while the provenance remains mandatory and inspectable.
type Parameter struct {
	Value              json.RawMessage    `json:"value"`
	ESS                *float64           `json:"ess,omitempty"`
	Source             string             `json:"source"`
	SourceURL          string             `json:"sourceUrl,omitempty"`
	RetrievedAt        string             `json:"retrievedAt"`
	VerificationStatus VerificationStatus `json:"verificationStatus"`
	Label              CalibrationLabel   `json:"label"`
	Notes              string             `json:"notes,omitempty"`
	Layer              string             `json:"-"`
}

type MeasuredStateSignal struct {
	Stage                 string    `json:"stage"`
	DeltaLogit            Parameter `json:"deltaLogit"`
	VarianceShare         Parameter `json:"varianceShare"`
	HeterogeneityArgument string    `json:"heterogeneityArgument"`
	DedupGroup            string    `json:"dedupGroup"`
	TauDays               float64   `json:"tauDays"`
	CapG                  float64   `json:"capG"`
}

type Pack struct {
	PackVersion          string                         `json:"packVersion"`
	SchemaVersion        int                            `json:"schemaVersion"`
	Sections             map[string]json.RawMessage     `json:"sections"`
	MeasuredStateSignals map[string]MeasuredStateSignal `json:"-"`
	Signature            string                         `json:"-"`
	Digest               string                         `json:"-"`
	Resolution           map[string]string              `json:"-"`
}

// Parameters returns a typed parameter section without exposing the pack's
// raw JSON representation to model consumers.
func (p *Pack) Parameters(section string) (map[string]Parameter, error) {
	raw, ok := p.Sections[section]
	if !ok {
		return nil, fmt.Errorf("risk pack section %q is absent", section)
	}
	var parameters map[string]Parameter
	if err := json.Unmarshal(raw, &parameters); err != nil {
		return nil, fmt.Errorf("decoding risk pack section %q: %w", section, err)
	}
	for key, parameter := range parameters {
		if err := validateParameter(section+"."+key, parameter); err != nil {
			return nil, err
		}
	}
	return parameters, nil
}

// MeasuredSignals returns the validated measured-state signal definitions.
func (p *Pack) MeasuredSignals() (map[string]MeasuredStateSignal, error) {
	if p.MeasuredStateSignals == nil {
		raw, ok := p.Sections["measuredStateSignals"]
		if !ok {
			return nil, errors.New("risk pack section \"measuredStateSignals\" is absent")
		}
		if err := json.Unmarshal(raw, &p.MeasuredStateSignals); err != nil {
			return nil, fmt.Errorf("decoding measuredStateSignals: %w", err)
		}
	}
	return p.MeasuredStateSignals, nil
}

// Load verifies and validates a base pack. The signature is detached and is
// over the exact bytes on disk, so formatting or parameter edits are visible.
func Load(path, signaturePath string, publicKey ed25519.PublicKey) (*Pack, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading risk pack: %w", err)
	}
	signature, err := os.ReadFile(signaturePath)
	if err != nil {
		return nil, fmt.Errorf("reading risk pack signature: %w", err)
	}
	if len(publicKey) != ed25519.PublicKeySize || !ed25519.Verify(publicKey, contents, signature) {
		return nil, errors.New("risk pack signature verification failed")
	}
	pack, err := ValidateBytes(contents)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(contents)
	pack.Digest = hex.EncodeToString(digest[:])
	pack.Signature = hex.EncodeToString(signature)
	return pack, nil
}

// ValidateBytes decodes and validates a pack without requiring a signature.
// Signing tools use this before producing a detached signature; consumers use
// Load, which verifies the signature before calling this function.
func ValidateBytes(contents []byte) (*Pack, error) {
	var pack Pack
	if err := json.Unmarshal(contents, &pack); err != nil {
		return nil, fmt.Errorf("decoding risk pack: %w", err)
	}
	if err := pack.Validate(); err != nil {
		return nil, err
	}
	return &pack, nil
}

// Resolve applies a separately supplied override layer without mutating the
// base. The resolved parameter names record which layer supplied each value.
func Resolve(base, override map[string]Parameter) map[string]Parameter {
	resolved := make(map[string]Parameter, len(base)+len(override))
	for key, value := range base {
		value.Layer = "base"
		resolved[key] = value
	}
	for key, value := range override {
		value.VerificationStatus = LocallyOverridden
		value.Layer = "override"
		resolved[key] = value
	}
	return resolved
}

func (p *Pack) Validate() error {
	if p.PackVersion == "" || p.SchemaVersion != schemaVersion || len(p.Sections) == 0 {
		return errors.New("risk pack has an invalid version or no sections")
	}
	for name, raw := range p.Sections {
		if err := validateParameters(name, raw); err != nil {
			return err
		}
	}
	if raw, ok := p.Sections["measuredStateSignals"]; ok {
		if err := json.Unmarshal(raw, &p.MeasuredStateSignals); err != nil {
			return fmt.Errorf("decoding measuredStateSignals: %w", err)
		}
		for id, signal := range p.MeasuredStateSignals {
			if err := validateParameter(fmt.Sprintf("measuredStateSignals.%s.deltaLogit", id), signal.DeltaLogit); err != nil {
				return err
			}
			if err := validateParameter(fmt.Sprintf("measuredStateSignals.%s.varianceShare", id), signal.VarianceShare); err != nil {
				return err
			}
			if signal.DedupGroup == "" || signal.TauDays <= 0 || signal.CapG < 1 ||
				signal.CapG > 2 || (parameterNumber(signal.VarianceShare) > 0 && signal.HeterogeneityArgument == "") {
				return fmt.Errorf("measured state signal %q is incomplete or invalid", id)
			}
		}
	}
	return nil
}

func validateParameters(section string, raw json.RawMessage) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("section %q must be an object: %w", section, err)
	}
	for key, value := range values {
		var object map[string]json.RawMessage
		if json.Unmarshal(value, &object) == nil && object["value"] != nil {
			var parameter Parameter
			if err := json.Unmarshal(value, &parameter); err != nil {
				return fmt.Errorf("parameter %s.%s lacks complete provenance", section, key)
			}
			if err := validateParameter(section+"."+key, parameter); err != nil {
				return err
			}
			continue
		}
		// Nested sections are allowed, but bare numeric leaves are not.
		if len(value) > 0 && (value[0] >= '0' && value[0] <= '9' || value[0] == '-') {
			return fmt.Errorf("parameter %s.%s is a bare value", section, key)
		}
	}
	return nil
}

func validateParameter(path string, parameter Parameter) error {
	if len(parameter.Value) == 0 || parameter.Source == "" || parameter.RetrievedAt == "" ||
		parameter.VerificationStatus == "" || parameter.Label == "" {
		return fmt.Errorf("parameter %s lacks complete provenance", path)
	}
	return nil
}

func parameterNumber(parameter Parameter) float64 {
	var value float64
	_ = json.Unmarshal(parameter.Value, &value)
	return value
}

type Observation struct {
	SignalID      string
	DedupGroup    string
	VarianceShare float64
	TauDays       float64
	CapG          float64
	ObservedAt    time.Time
	Assessed      bool
}

type PrecisionResult struct {
	Gain       float64
	Groups     []string
	CapApplied bool
}

// PrecisionGain computes the current measured-state gain. It has no history
// input by design: observing an unchanged configuration repeatedly cannot
// accumulate susceptibility evidence.
func PrecisionGain(observations []Observation, now time.Time, stageCap float64) (PrecisionResult, error) {
	if stageCap < 1 || stageCap > 2 {
		return PrecisionResult{}, fmt.Errorf("stage precision cap %v is outside [1, 2]", stageCap)
	}
	shares := map[string]float64{}
	for _, observation := range observations {
		if !observation.Assessed || observation.DedupGroup == "" || observation.TauDays <= 0 {
			continue
		}
		ageDays := now.Sub(observation.ObservedAt).Hours() / 24
		if ageDays < 0 {
			ageDays = 0
		}
		decayed := observation.VarianceShare * math.Exp(-ageDays/observation.TauDays)
		if decayed > shares[observation.DedupGroup] {
			shares[observation.DedupGroup] = decayed
		}
	}
	groups := make([]string, 0, len(shares))
	total := float64(0)
	for group, share := range shares {
		groups = append(groups, group)
		total += share
	}
	sort.Strings(groups)
	if total >= 1 {
		total = math.Nextafter(1, 0)
	}
	gain := 1 / (1 - total)
	return PrecisionResult{Gain: math.Min(gain, stageCap), Groups: groups, CapApplied: gain > stageCap}, nil
}

func ConditionedESS(classESS, gain float64) (float64, error) {
	if classESS < 0 || gain < 1 {
		return 0, errors.New("effective sample size and gain must be non-negative and gain must be at least one")
	}
	return (classESS+1)*gain - 1, nil
}

func (p *Pack) Parameter(path string) (Parameter, bool) {
	parts := strings.Split(path, ".")
	if len(parts) != 2 {
		return Parameter{}, false
	}
	var values map[string]Parameter
	raw, ok := p.Sections[parts[0]]
	if !ok || json.Unmarshal(raw, &values) != nil {
		return Parameter{}, false
	}
	value, ok := values[parts[1]]
	return value, ok
}

// Package contact contains the exposure-side primitives for Change 007.
// Scanner observations describe reachability and exposure, never attacker
// contact; contact posterior telemetry belongs to a separate future path.
package contact

import (
	"fmt"
	"math"
	"sort"
	"time"
)

func expNeg(x float64) float64 {
	if x <= 0 {
		return 1
	}
	return math.Exp(-x)
}

type ExposureState string

const (
	Exposed ExposureState = "exposed"
	Clear   ExposureState = "clear"
	Unknown ExposureState = "unknown"
)

type Observation struct {
	At         time.Time
	State      ExposureState
	Successful bool
}

type History struct {
	FirstObserved     time.Time
	LastObserved      time.Time
	StillExposed      bool
	LeftCensored      bool
	ObservedDuration  time.Duration
	InferredDuration  time.Duration
	BlindDuration     time.Duration
	AwareObserved     time.Duration
	AwareInferred     time.Duration
	ExpectedBlindTime time.Duration
	WorstBlindTime    time.Duration
}

// ComputeHistory accounts for an exposure window without collapsing observed
// and inferred time. A failed or unassessed observation cannot close an open
// exposure, and time after the last successful boundary is inferred.
func ComputeHistory(observations []Observation, now time.Time, cadence time.Duration) (History, error) {
	if cadence <= 0 {
		return History{}, fmt.Errorf("assessment cadence must be positive")
	}
	if now.IsZero() {
		now = time.Now()
	}
	ordered := append([]Observation(nil), observations...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].At.Before(ordered[j].At) })

	result := History{
		ExpectedBlindTime: cadence / 2,
		WorstBlindTime:    cadence,
	}
	var openAt, previousAt time.Time
	previousSuccessful := false
	previousState := Unknown
	open := false

	for _, observation := range ordered {
		if observation.At.IsZero() || observation.At.After(now) {
			continue
		}
		if !open {
			if observation.State != Exposed {
				previousAt, previousSuccessful, previousState = observation.At, observation.Successful, observation.State
				continue
			}
			open = true
			openAt = observation.At
			result.FirstObserved = observation.At
			result.LeftCensored = true
			result.BlindDuration = result.ExpectedBlindTime
			previousAt, previousSuccessful, previousState = observation.At, observation.Successful, observation.State
			continue
		}

		interval := observation.At.Sub(previousAt)
		if interval > 0 {
			if previousSuccessful && observation.Successful && previousState == Exposed {
				result.ObservedDuration += interval
			} else {
				result.InferredDuration += interval
			}
		}

		if observation.Successful && observation.State == Clear {
			open = false
			result.LastObserved = observation.At
		} else {
			result.LastObserved = observation.At
		}
		previousAt, previousSuccessful, previousState = observation.At, observation.Successful, observation.State
	}

	if !open {
		result.AwareObserved = result.ObservedDuration
		result.AwareInferred = result.InferredDuration
		return result, nil
	}

	result.StillExposed = true
	result.LastObserved = previousAt
	if previousAt.Before(now) {
		// No successful assessment bounds the open interval after the latest
		// observation, so it is inferred even when the latest state was exposed.
		result.InferredDuration += now.Sub(previousAt)
	}
	result.AwareObserved = result.ObservedDuration
	result.AwareInferred = result.InferredDuration
	_ = openAt
	return result, nil
}

// LiveTimeAtRisk advances a stored or previously computed History to wallClock
// without collapsing observed and inferred intervals. For an exposure that is
// still open past LastObserved, elapsed wall-clock time accrues into
// InferredDuration (and AwareInferred), because no assessment has bounded the
// trailing open interval yet.
func (h History) LiveTimeAtRisk(wallClock time.Time) History {
	out := h
	if !out.StillExposed || out.LastObserved.IsZero() || !wallClock.After(out.LastObserved) {
		return out
	}
	trailing := wallClock.Sub(out.LastObserved)
	// ComputeHistory may already have accrued up to an earlier `now`; when a
	// stored row is rehydrated from ObservedDuration + LastObserved, callers set
	// InferredDuration to the historical gap before LastObserved and call
	// LiveTimeAtRisk(now). To stay idempotent only when RehydrateHistory is used,
	// RehydrateHistory constructs the pre-trailing History cleanly.
	out.InferredDuration += trailing
	out.AwareInferred = out.InferredDuration
	return out
}

// TotalAwareDays returns aware time at risk (observed + inferred) in days, kept
// as an internal arithmetic helper for rate evaluation while keeping the two
// durations separate on every emitted type.
func (h History) TotalAwareDays() float64 {
	return (h.ObservedDuration + h.InferredDuration).Hours() / 24.0
}

// CadenceComparison reports the expected blind-time and contact-probability
// reduction from moving an asset class from currentCadence to proposedCadence,
// given an observed weakness-introduction rate (introductions per year) and an
// effective contact rate lambdaPerDay.
type CadenceComparison struct {
	CurrentCadence           time.Duration `json:"currentCadence"`
	ProposedCadence          time.Duration `json:"proposedCadence"`
	CurrentExpectedBlind     time.Duration `json:"currentExpectedBlind"`
	ProposedExpectedBlind    time.Duration `json:"proposedExpectedBlind"`
	BlindTimeReduction       time.Duration `json:"blindTimeReduction"`
	CurrentBlindContactProb  float64       `json:"currentBlindContactProb"`
	ProposedBlindContactProb float64       `json:"proposedBlindContactProb"`
	ContactProbReduction     float64       `json:"contactProbReduction"`
	ExpectedBlindDaysSavedYr float64       `json:"expectedBlindDaysSavedPerYear"`
}

// EvaluateCadenceChange computes the exposure reduction from changing the
// assessment cadence, given the observed weakness-introduction rate per year
// and the daily contact rate lambdaPerDay.
func EvaluateCadenceChange(currentCadence, proposedCadence time.Duration, introductionsPerYear, lambdaPerDay float64) (CadenceComparison, error) {
	if currentCadence <= 0 || proposedCadence <= 0 {
		return CadenceComparison{}, fmt.Errorf("cadences must be positive")
	}
	if introductionsPerYear < 0 || lambdaPerDay < 0 {
		return CadenceComparison{}, fmt.Errorf("rates must be non-negative")
	}
	curBlind := currentCadence / 2
	propBlind := proposedCadence / 2
	curDays := curBlind.Hours() / 24.0
	propDays := propBlind.Hours() / 24.0
	curProb := 1.0 - expNeg(lambdaPerDay*curDays)
	propProb := 1.0 - expNeg(lambdaPerDay*propDays)
	return CadenceComparison{
		CurrentCadence:           currentCadence,
		ProposedCadence:          proposedCadence,
		CurrentExpectedBlind:     curBlind,
		ProposedExpectedBlind:    propBlind,
		BlindTimeReduction:       curBlind - propBlind,
		CurrentBlindContactProb:  curProb,
		ProposedBlindContactProb: propProb,
		ContactProbReduction:     curProb - propProb,
		ExpectedBlindDaysSavedYr: introductionsPerYear * (curDays - propDays),
	}, nil
}

// LatencySplit reports detection latency (a property of the monitoring
// programme's cadence) separately from remediation latency (time from first
// detection to confirmed clean closure). They are never summed into a single
// responsiveness metric.
type LatencySplit struct {
	ExpectedDetectionLatency time.Duration `json:"expectedDetectionLatency"`
	WorstDetectionLatency    time.Duration `json:"worstDetectionLatency"`
	RemediationLatency       time.Duration `json:"remediationLatency"`
	StillOpen                bool          `json:"stillOpen"`
}

// ComputeLatencySplit derives separate detection and remediation latencies from
// an exposure history and wall clock.
func ComputeLatencySplit(h History, now time.Time) LatencySplit {
	var rem time.Duration
	if !h.FirstObserved.IsZero() {
		if h.StillExposed {
			if now.After(h.FirstObserved) {
				rem = now.Sub(h.FirstObserved)
			}
		} else if h.LastObserved.After(h.FirstObserved) {
			rem = h.LastObserved.Sub(h.FirstObserved)
		}
	}
	return LatencySplit{
		ExpectedDetectionLatency: h.ExpectedBlindTime,
		WorstDetectionLatency:    h.WorstBlindTime,
		RemediationLatency:       rem,
		StillOpen:                h.StillExposed,
	}
}

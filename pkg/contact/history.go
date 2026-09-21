// Package contact contains the exposure-side primitives for Change 007.
// Scanner observations describe reachability and exposure, never attacker
// contact; contact posterior telemetry belongs to a separate future path.
package contact

import (
	"fmt"
	"sort"
	"time"
)

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

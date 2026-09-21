package contact

import (
	"testing"
	"time"
)

func TestContinuousExposureSeparatesObservedAndInferredTime(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start.Add(48 * time.Hour)
	result, err := ComputeHistory([]Observation{
		{At: start, State: Exposed, Successful: true},
		{At: start.Add(24 * time.Hour), State: Exposed, Successful: true},
		{At: now, State: Exposed, Successful: true},
	}, now, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !result.StillExposed || result.ObservedDuration != 48*time.Hour || result.InferredDuration != 0 {
		t.Fatalf("continuous history was not observed: %+v", result)
	}
}

func TestAssessmentGapIsInferredAndCleanClosesExposure(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := ComputeHistory([]Observation{
		{At: start, State: Exposed, Successful: true},
		{At: start.Add(24 * time.Hour), State: Exposed, Successful: false},
		{At: start.Add(96 * time.Hour), State: Clear, Successful: true},
	}, start.Add(96*time.Hour), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if result.StillExposed || result.ObservedDuration != 0 || result.InferredDuration != 96*time.Hour {
		t.Fatalf("gap accounting or closure was wrong: %+v", result)
	}
}

func TestFirstExposureIsLeftCensoredAndBlindTimeIsEstimated(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := ComputeHistory([]Observation{{At: start, State: Exposed, Successful: true}}, start, 90*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !result.LeftCensored || result.BlindDuration != 45*24*time.Hour || result.ExpectedBlindTime != result.BlindDuration {
		t.Fatalf("blind-time disclosure was wrong: %+v", result)
	}
}

func TestUnknownObservationDoesNotCloseExposure(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := ComputeHistory([]Observation{
		{At: start, State: Exposed, Successful: true},
		{At: start.Add(24 * time.Hour), State: Unknown, Successful: false},
	}, start.Add(48*time.Hour), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !result.StillExposed || result.InferredDuration != 48*time.Hour {
		t.Fatalf("unknown observation closed or undercounted exposure: %+v", result)
	}
}

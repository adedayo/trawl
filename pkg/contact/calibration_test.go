package contact

import (
	"testing"
	"time"
)

func TestCalibrationObservationRequiresRedactedSourceKey(t *testing.T) {
	observation := CalibrationObservation{
		ExperimentID: "experiment-1", EndpointID: "endpoint-1",
		ServiceClass: ServiceClassSSH, Transport: "tcp", Port: 22,
		ObservedAt: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
		EventType:  ConnectionAttempt,
	}
	if err := observation.Validate(); err == nil {
		t.Fatal("observation without source key was accepted")
	}
	observation.SourceKey, _ = NewSourceKey("rotation-secret", "source-address")
	if err := observation.Validate(); err != nil {
		t.Fatalf("valid observation rejected: %v", err)
	}
}

func TestCalibrationObservationRejectsUnknownServiceClass(t *testing.T) {
	observation := CalibrationObservation{
		ID: "observation-1", ExperimentID: "experiment-1", EndpointID: "endpoint-1",
		ServiceClass: ServiceClass("unreviewed-service"), Transport: "tcp", Port: 1,
		ObservedAt: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
		EventType:  ConnectionAttempt, SourceKey: SourceKey("hmac-digest"),
	}
	if err := observation.Validate(); err == nil {
		t.Fatal("unknown service class was accepted")
	}
}

func TestNewSourceKeyIsKeyedAndDeterministic(t *testing.T) {
	first, err := NewSourceKey("key-a", "source-address")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSourceKey("key-a", "source-address")
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := NewSourceKey("key-b", "source-address")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first == otherKey {
		t.Fatal("source key does not have the expected keyed deterministic behaviour")
	}
}

func TestCalibrationCollectorRedactsSourceBeforeEmittingObservation(t *testing.T) {
	collector, err := NewCalibrationCollector("experiment-1", "endpoint-1", ServiceClassSSH, "tcp", 22, "rotation-secret")
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	observation, err := collector.Observe("198.51.100.10", ConnectionAttempt, observedAt, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if observation.SourceKey == "198.51.100.10" || observation.SourceKey == "" {
		t.Fatalf("source was not redacted: %q", observation.SourceKey)
	}
	if observation.ID == "" || observation.ExperimentID != "experiment-1" || observation.ObservedAt != observedAt {
		t.Fatalf("collector emitted incomplete observation: %+v", observation)
	}
}

func TestCalibrationCollectorRejectsIncompleteConfiguration(t *testing.T) {
	if _, err := NewCalibrationCollector("", "endpoint-1", ServiceClassSSH, "tcp", 22, "key"); err == nil {
		t.Fatal("incomplete collector configuration was accepted")
	}
}

func TestCalibrationCollectorEmitsExperimentScopedReachability(t *testing.T) {
	collector, err := NewCalibrationCollector("experiment-1", "endpoint-1", ServiceClassWeb, "tcp", 443, "key")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	interval, err := collector.Reachability(start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if interval.ExperimentID != "experiment-1" || interval.EndpointID != "endpoint-1" {
		t.Fatalf("interval lost collector scope: %+v", interval)
	}
}

func TestReachabilityIntervalProducesServiceDays(t *testing.T) {
	interval := ReachabilityInterval{
		ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassWeb,
		StartedAt: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
	}
	days, err := interval.ServiceDays()
	if err != nil {
		t.Fatal(err)
	}
	if days != 2.5 {
		t.Fatalf("service days = %v, want 2.5", days)
	}
}

func TestContactRateRequiresAReachabilityDenominator(t *testing.T) {
	if _, err := ContactRate(1, 0); err == nil {
		t.Fatal("zero exposure denominator was accepted")
	}
	rate, err := ContactRate(10, 2.5)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 4 {
		t.Fatalf("rate = %v, want 4", rate)
	}
}

func TestPoissonRateIntervalKeepsZeroCountsUncertain(t *testing.T) {
	lower, upper, err := PoissonRateInterval(0, 1, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if lower != 0 || upper < 3.68 || upper > 3.70 {
		t.Fatalf("zero-count interval = [%v, %v], want approximately [0, 3.689]", lower, upper)
	}
}

func TestPoissonRateIntervalContainsObservedRate(t *testing.T) {
	rate, err := ContactRate(10, 2.5)
	if err != nil {
		t.Fatal(err)
	}
	lower, upper, err := PoissonRateInterval(10, 2.5, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if lower <= 0 || lower >= rate || upper <= rate {
		t.Fatalf("interval = [%v, %v], rate = %v", lower, upper, rate)
	}
}

func TestAggregateCalibrationRetainsZeroEventTypes(t *testing.T) {
	start := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	buckets, err := AggregateCalibration(nil, []ReachabilityInterval{{
		ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassRDP,
		StartedAt: start, EndedAt: start.Add(24 * time.Hour),
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []CalibrationEventType{ConnectionAttempt, ProtocolInteraction, ApplicationRequest} {
		bucket, ok := buckets[CalibrationBucketKey{ServiceClass: ServiceClassRDP, EventType: eventType}]
		if !ok || bucket.EventCount != 0 || bucket.ReachableServiceDays != 1 {
			t.Fatalf("zero event type %q was omitted or malformed: %+v", eventType, bucket)
		}
	}
}

func TestAggregateCalibrationKeepsEventTypesSeparate(t *testing.T) {
	start := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	sourceKey, err := NewSourceKey("rotation-secret", "source-address")
	if err != nil {
		t.Fatal(err)
	}
	observations := []CalibrationObservation{
		{ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassSSH, Transport: "tcp", Port: 22, ObservedAt: start, EventType: ConnectionAttempt, SourceKey: sourceKey},
		{ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassSSH, Transport: "tcp", Port: 22, ObservedAt: start.Add(time.Hour), EventType: ConnectionAttempt, SourceKey: sourceKey},
		{ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassSSH, Transport: "tcp", Port: 22, ObservedAt: start.Add(2 * time.Hour), EventType: ProtocolInteraction, SourceKey: sourceKey},
	}
	buckets, err := AggregateCalibration(observations, []ReachabilityInterval{{
		ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassSSH,
		StartedAt: start, EndedAt: start.Add(48 * time.Hour),
	}})
	if err != nil {
		t.Fatal(err)
	}
	sshConnection := buckets[CalibrationBucketKey{ServiceClass: ServiceClassSSH, EventType: ConnectionAttempt}]
	sshProtocol := buckets[CalibrationBucketKey{ServiceClass: ServiceClassSSH, EventType: ProtocolInteraction}]
	if sshConnection.EventCount != 2 || sshProtocol.EventCount != 1 {
		t.Fatalf("event types were combined: %+v", buckets)
	}
	if sshConnection.ReachableServiceDays != 2 || sshConnection.EndpointCount != 1 {
		t.Fatalf("denominator metadata was lost: %+v", sshConnection)
	}
	if sshConnection.UniqueSourceCount != 1 {
		t.Fatalf("repeated source was counted as multiple sources: %+v", sshConnection)
	}
	if !sshConnection.ObservationStart.Equal(start) || !sshConnection.ObservationEnd.Equal(start.Add(48*time.Hour)) {
		t.Fatalf("observation window was lost: %+v", sshConnection)
	}

	webObservation := observations[0]
	webObservation.ID = "web-observation"
	webObservation.ServiceClass = ServiceClassWeb
	mixedIntervals := []ReachabilityInterval{
		{
			ExperimentID: "experiment-1", EndpointID: "endpoint-2", ServiceClass: ServiceClassWeb,
			StartedAt: start, EndedAt: start.Add(24 * time.Hour),
		},
		{
			ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassSSH,
			StartedAt: start, EndedAt: start.Add(48 * time.Hour),
		},
	}
	webBuckets, err := AggregateCalibration(append(observations, webObservation), mixedIntervals)
	if err != nil {
		t.Fatal(err)
	}
	webConnection := webBuckets[CalibrationBucketKey{ServiceClass: ServiceClassWeb, EventType: ConnectionAttempt}]
	if webConnection.EventCount != 1 || webConnection.ReachableServiceDays != 1 {
		t.Fatalf("mixed service classes were combined: %+v", webBuckets)
	}
}

func TestAggregateCalibrationRejectsOverlappingIntervals(t *testing.T) {
	start := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	_, err := AggregateCalibration(nil, []ReachabilityInterval{
		{ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassWeb, StartedAt: start, EndedAt: start.Add(24 * time.Hour)},
		{ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassWeb, StartedAt: start.Add(12 * time.Hour), EndedAt: start.Add(36 * time.Hour)},
	})
	if err == nil {
		t.Fatal("overlapping reachability intervals were accepted")
	}
}

func TestAggregateCalibrationRejectsMixedExperiments(t *testing.T) {
	start := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	_, err := AggregateCalibration(nil, []ReachabilityInterval{
		{ExperimentID: "experiment-1", EndpointID: "endpoint-1", ServiceClass: ServiceClassWeb, StartedAt: start, EndedAt: start.Add(24 * time.Hour)},
		{ExperimentID: "experiment-2", EndpointID: "endpoint-2", ServiceClass: ServiceClassWeb, StartedAt: start, EndedAt: start.Add(24 * time.Hour)},
	})
	if err == nil {
		t.Fatal("mixed experiments were accepted")
	}
}

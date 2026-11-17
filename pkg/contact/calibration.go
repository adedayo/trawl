package contact

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

type CalibrationEventType string

const (
	ConnectionAttempt   CalibrationEventType = "connection-attempt"
	ProtocolInteraction CalibrationEventType = "protocol-interaction"
	ApplicationRequest  CalibrationEventType = "application-request"
)

type SourceKey string

type CalibrationCollector struct {
	experimentID string
	endpointID   string
	serviceClass ServiceClass
	transport    string
	port         uint16
	key          string
}

func NewCalibrationCollector(experimentID, endpointID string, serviceClass ServiceClass, transport string, port uint16, key string) (*CalibrationCollector, error) {
	collector := &CalibrationCollector{
		experimentID: experimentID,
		endpointID:   endpointID,
		serviceClass: serviceClass,
		transport:    transport,
		port:         port,
		key:          key,
	}
	if experimentID == "" || endpointID == "" || serviceClass == "" || transport == "" || port == 0 || key == "" {
		return nil, errors.New("calibration collector configuration is incomplete")
	}
	return collector, nil
}

func (c *CalibrationCollector) Observe(source string, eventType CalibrationEventType, observedAt time.Time, accepted, protocolIdentified bool) (CalibrationObservation, error) {
	if c == nil {
		return CalibrationObservation{}, errors.New("calibration collector is nil")
	}
	sourceKey, err := NewSourceKey(c.key, source)
	if err != nil {
		return CalibrationObservation{}, err
	}
	id, err := newCalibrationID()
	if err != nil {
		return CalibrationObservation{}, err
	}
	observation := CalibrationObservation{
		ID:                 id,
		ExperimentID:       c.experimentID,
		EndpointID:         c.endpointID,
		ServiceClass:       c.serviceClass,
		Transport:          c.transport,
		Port:               c.port,
		ObservedAt:         observedAt,
		EventType:          eventType,
		SourceKey:          sourceKey,
		Accepted:           accepted,
		ProtocolIdentified: protocolIdentified,
	}
	if err := observation.Validate(); err != nil {
		return CalibrationObservation{}, err
	}
	return observation, nil
}

func (c *CalibrationCollector) Reachability(startedAt, endedAt time.Time) (ReachabilityInterval, error) {
	if c == nil {
		return ReachabilityInterval{}, errors.New("calibration collector is nil")
	}
	interval := ReachabilityInterval{
		ExperimentID: c.experimentID,
		EndpointID:   c.endpointID,
		ServiceClass: c.serviceClass,
		StartedAt:    startedAt,
		EndedAt:      endedAt,
	}
	if err := interval.Validate(); err != nil {
		return ReachabilityInterval{}, err
	}
	return interval, nil
}

func newCalibrationID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generating calibration observation id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func NewSourceKey(key, source string) (SourceKey, error) {
	if key == "" || source == "" {
		return "", errors.New("source key requires a non-empty key and source")
	}
	digest := hmac.New(sha256.New, []byte(key))
	_, _ = digest.Write([]byte(source))
	return SourceKey(hex.EncodeToString(digest.Sum(nil))), nil
}

type CalibrationObservation struct {
	ID                 string
	ExperimentID       string
	EndpointID         string
	ServiceClass       ServiceClass
	Transport          string
	Port               uint16
	ObservedAt         time.Time
	EventType          CalibrationEventType
	SourceKey          SourceKey
	Accepted           bool
	ProtocolIdentified bool
}

func (o CalibrationObservation) Validate() error {
	if o.ExperimentID == "" || o.EndpointID == "" {
		return errors.New("calibration observation requires experiment and endpoint identifiers")
	}
	if !IsKnownServiceClass(o.ServiceClass) {
		return fmt.Errorf("unknown calibration service class %q", o.ServiceClass)
	}
	if o.Transport == "" || o.Port == 0 {
		return errors.New("calibration observation requires transport and port")
	}
	if o.ObservedAt.IsZero() {
		return errors.New("calibration observation requires an observation time")
	}
	if o.EventType != ConnectionAttempt && o.EventType != ProtocolInteraction && o.EventType != ApplicationRequest {
		return fmt.Errorf("unsupported calibration event type %q", o.EventType)
	}
	if o.SourceKey == "" {
		return errors.New("calibration observation requires a redacted source key")
	}
	return nil
}

type ReachabilityInterval struct {
	ExperimentID string
	EndpointID   string
	ServiceClass ServiceClass
	StartedAt    time.Time
	EndedAt      time.Time
}

func (i ReachabilityInterval) Validate() error {
	if i.ExperimentID == "" || i.EndpointID == "" || !IsKnownServiceClass(i.ServiceClass) {
		return errors.New("reachability interval requires experiment, endpoint, and service class")
	}
	if i.StartedAt.IsZero() || i.EndedAt.IsZero() || !i.EndedAt.After(i.StartedAt) {
		return errors.New("reachability interval must have an ordered non-zero duration")
	}
	return nil
}

func (i ReachabilityInterval) ServiceDays() (float64, error) {
	if err := i.Validate(); err != nil {
		return 0, err
	}
	return i.EndedAt.Sub(i.StartedAt).Hours() / 24, nil
}

func ContactRate(eventCount uint64, reachableServiceDays float64) (float64, error) {
	if reachableServiceDays <= 0 {
		return 0, errors.New("reachable service-days must be positive")
	}
	return float64(eventCount) / reachableServiceDays, nil
}

// PoissonRateInterval returns an exact Garwood interval for a Poisson rate.
// The interval is computed by inverting the Poisson CDF, so zero events retain
// a non-zero upper bound rather than being reported as no ambient attention.
func PoissonRateInterval(eventCount uint64, reachableServiceDays, confidence float64) (float64, float64, error) {
	if reachableServiceDays <= 0 {
		return 0, 0, errors.New("reachable service-days must be positive")
	}
	if confidence <= 0 || confidence >= 1 || math.IsNaN(confidence) {
		return 0, 0, errors.New("confidence must be between zero and one")
	}
	alpha := 1 - confidence
	if eventCount == 0 {
		upper := poissonUpperMean(0, alpha/2)
		return 0, upper / reachableServiceDays, nil
	}
	lower := poissonLowerMean(eventCount, 1-alpha/2)
	upper := poissonUpperMean(eventCount, alpha/2)
	return lower / reachableServiceDays, upper / reachableServiceDays, nil
}

func poissonLowerMean(eventCount uint64, target float64) float64 {
	low, high := 0.0, math.Max(1, float64(eventCount))
	for iteration := 0; iteration < 100; iteration++ {
		mid := (low + high) / 2
		if poissonCDF(eventCount-1, mid) > target {
			low = mid
		} else {
			high = mid
		}
	}
	return high
}

func poissonUpperMean(eventCount uint64, target float64) float64 {
	low, high := 0.0, math.Max(1, float64(eventCount+1))
	for poissonCDF(eventCount, high) > target {
		high *= 2
	}
	for iteration := 0; iteration < 100; iteration++ {
		mid := (low + high) / 2
		if poissonCDF(eventCount, mid) > target {
			low = mid
		} else {
			high = mid
		}
	}
	return high
}

func poissonCDF(eventCount uint64, mean float64) float64 {
	term := math.Exp(-mean)
	sum := term
	for index := uint64(1); index <= eventCount; index++ {
		term *= mean / float64(index)
		sum += term
		if term == 0 {
			break
		}
	}
	return sum
}

type CalibrationBucket struct {
	ServiceClass         ServiceClass
	EventType            CalibrationEventType
	EventCount           uint64
	ReachableServiceDays float64
	EndpointCount        int
	UniqueSourceCount    int
	ObservationStart     time.Time
	ObservationEnd       time.Time
}

type CalibrationBucketKey struct {
	ServiceClass ServiceClass
	EventType    CalibrationEventType
}

type CalibrationReportRow struct {
	ExperimentID          string
	ServiceClass          string
	EventType             string
	EventCount            uint64
	ReachableServiceDays  float64
	EndpointCount         int
	UniqueSourceCount     int
	RatePerServiceDay     float64
	RateLowPerServiceDay  float64
	RateHighPerServiceDay float64
	IntervalMethod        string
	Confidence            float64
	ObservationStart      string
	ObservationEnd        string
	DeduplicationRule     string
}

func (b CalibrationBucket) Rate() (float64, error) {
	return ContactRate(b.EventCount, b.ReachableServiceDays)
}

func AggregateCalibration(observations []CalibrationObservation, intervals []ReachabilityInterval) (map[CalibrationBucketKey]CalibrationBucket, error) {
	experimentID := ""
	for _, interval := range intervals {
		if err := interval.Validate(); err != nil {
			return nil, err
		}
		if experimentID == "" {
			experimentID = interval.ExperimentID
		} else if interval.ExperimentID != experimentID {
			return nil, fmt.Errorf("calibration records belong to multiple experiments: %q and %q", experimentID, interval.ExperimentID)
		}
	}
	for _, observation := range observations {
		if err := observation.Validate(); err != nil {
			return nil, err
		}
		if experimentID == "" {
			experimentID = observation.ExperimentID
		} else if observation.ExperimentID != experimentID {
			return nil, fmt.Errorf("calibration records belong to multiple experiments: %q and %q", experimentID, observation.ExperimentID)
		}
	}
	intervalDays, endpointCount, observationStart, observationEnd, err := aggregateReachability(intervals)
	if err != nil {
		return nil, err
	}
	buckets := make(map[CalibrationBucketKey]CalibrationBucket)
	sourceKeys := make(map[CalibrationBucketKey]map[SourceKey]struct{})
	eventTypes := []CalibrationEventType{ConnectionAttempt, ProtocolInteraction, ApplicationRequest}
	for serviceClass, days := range intervalDays {
		for _, eventType := range eventTypes {
			key := CalibrationBucketKey{ServiceClass: serviceClass, EventType: eventType}
			buckets[key] = CalibrationBucket{
				ServiceClass: serviceClass, EventType: eventType,
				ReachableServiceDays: days, EndpointCount: endpointCount[serviceClass],
				ObservationStart: observationStart[serviceClass], ObservationEnd: observationEnd[serviceClass],
			}
		}
	}
	for _, observation := range observations {
		key := CalibrationBucketKey{ServiceClass: observation.ServiceClass, EventType: observation.EventType}
		bucket := buckets[key]
		bucket.ServiceClass = observation.ServiceClass
		bucket.EventType = observation.EventType
		bucket.EventCount++
		bucket.ReachableServiceDays = intervalDays[observation.ServiceClass]
		bucket.EndpointCount = endpointCount[observation.ServiceClass]
		bucket.ObservationStart = observationStart[observation.ServiceClass]
		bucket.ObservationEnd = observationEnd[observation.ServiceClass]
		if sourceKeys[key] == nil {
			sourceKeys[key] = make(map[SourceKey]struct{})
		}
		sourceKeys[key][observation.SourceKey] = struct{}{}
		bucket.UniqueSourceCount = len(sourceKeys[key])
		buckets[key] = bucket
	}
	return buckets, nil
}

func aggregateReachability(intervals []ReachabilityInterval) (map[ServiceClass]float64, map[ServiceClass]int, map[ServiceClass]time.Time, map[ServiceClass]time.Time, error) {
	byEndpoint := make(map[string][]ReachabilityInterval)
	for _, interval := range intervals {
		if err := interval.Validate(); err != nil {
			return nil, nil, nil, nil, err
		}
		byEndpoint[interval.EndpointID] = append(byEndpoint[interval.EndpointID], interval)
	}
	days := make(map[ServiceClass]float64)
	endpoints := make(map[ServiceClass]map[string]struct{})
	starts := make(map[ServiceClass]time.Time)
	ends := make(map[ServiceClass]time.Time)
	for endpointID, endpointIntervals := range byEndpoint {
		sort.Slice(endpointIntervals, func(i, j int) bool {
			return endpointIntervals[i].StartedAt.Before(endpointIntervals[j].StartedAt)
		})
		for index, interval := range endpointIntervals {
			if index > 0 && !interval.StartedAt.Before(endpointIntervals[index-1].EndedAt) {
				continue
			}
			if index > 0 && interval.StartedAt.Before(endpointIntervals[index-1].EndedAt) {
				return nil, nil, nil, nil, fmt.Errorf("reachability intervals overlap for endpoint %q", endpointID)
			}
			intervalDuration, err := interval.ServiceDays()
			if err != nil {
				return nil, nil, nil, nil, err
			}
			days[interval.ServiceClass] += intervalDuration
			if endpoints[interval.ServiceClass] == nil {
				endpoints[interval.ServiceClass] = make(map[string]struct{})
			}
			endpoints[interval.ServiceClass][endpointID] = struct{}{}
			if starts[interval.ServiceClass].IsZero() || interval.StartedAt.Before(starts[interval.ServiceClass]) {
				starts[interval.ServiceClass] = interval.StartedAt
			}
			if interval.EndedAt.After(ends[interval.ServiceClass]) {
				ends[interval.ServiceClass] = interval.EndedAt
			}
		}
	}
	counts := make(map[ServiceClass]int, len(endpoints))
	for serviceClass, endpointIDs := range endpoints {
		counts[serviceClass] = len(endpointIDs)
	}
	return days, counts, starts, ends, nil
}

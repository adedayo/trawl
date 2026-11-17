// Command calibration-report summarizes one persisted ambient-contact
// experiment. It reports observed rates only; it does not create or promote a
// model-pack parameter.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/store"
	_ "github.com/adedayo/trawl/pkg/store/sqlite"
)

type reportRow struct {
	ExperimentID          string  `json:"experimentId"`
	ServiceClass          string  `json:"serviceClass"`
	EventType             string  `json:"eventType"`
	EventCount            uint64  `json:"eventCount"`
	ReachableServiceDays  float64 `json:"reachableServiceDays"`
	EndpointCount         int     `json:"endpointCount"`
	UniqueSourceCount     int     `json:"uniqueSourceCount"`
	RatePerServiceDay     float64 `json:"ratePerServiceDay"`
	RateLowPerServiceDay  float64 `json:"rateLowPerServiceDay"`
	RateHighPerServiceDay float64 `json:"rateHighPerServiceDay"`
	IntervalMethod        string  `json:"intervalMethod"`
	Confidence            float64 `json:"confidence"`
	ObservationStart      string  `json:"observationStart"`
	ObservationEnd        string  `json:"observationEnd"`
	DeduplicationRule     string  `json:"deduplicationRule"`
}

func main() {
	db := flag.String("db", "", "SQLite database path")
	experimentID := flag.String("experiment", "", "calibration experiment identifier")
	confidence := flag.Float64("confidence", 0.95, "interval confidence between zero and one")
	flag.Parse()
	if *experimentID == "" {
		fail("--experiment is required")
	}

	dbStore, err := store.Open(*db)
	if err != nil {
		fail(err.Error())
	}
	defer dbStore.Close()

	observations, err := dbStore.GetCalibrationObservations(context.Background(), *experimentID)
	if err != nil {
		fail(err.Error())
	}
	intervals, err := dbStore.GetCalibrationReachability(context.Background(), *experimentID)
	if err != nil {
		fail(err.Error())
	}

	contactObservations := make([]contact.CalibrationObservation, 0, len(observations))
	for _, observation := range observations {
		contactObservations = append(contactObservations, contact.CalibrationObservation{
			ID: observation.ID, ExperimentID: observation.ExperimentID,
			EndpointID: observation.EndpointID, ServiceClass: contact.ServiceClass(observation.ServiceClass),
			Transport: observation.Transport, Port: observation.Port,
			ObservedAt: observation.ObservedAt, EventType: contact.CalibrationEventType(observation.EventType),
			SourceKey: contact.SourceKey(observation.SourceKey), Accepted: observation.Accepted,
			ProtocolIdentified: observation.ProtocolIdentified,
		})
	}
	contactIntervals := make([]contact.ReachabilityInterval, 0, len(intervals))
	for _, interval := range intervals {
		contactIntervals = append(contactIntervals, contact.ReachabilityInterval{
			ExperimentID: interval.ExperimentID, EndpointID: interval.EndpointID,
			ServiceClass: contact.ServiceClass(interval.ServiceClass),
			StartedAt:    interval.StartedAt, EndedAt: interval.EndedAt,
		})
	}

	buckets, err := contact.AggregateCalibration(contactObservations, contactIntervals)
	if err != nil {
		fail(err.Error())
	}
	rows := make([]reportRow, 0, len(buckets))
	for key, bucket := range buckets {
		rate, rateErr := bucket.Rate()
		if rateErr != nil {
			fail(fmt.Sprintf("%s/%s: %v", key.ServiceClass, key.EventType, rateErr))
		}
		low, high, intervalErr := contact.PoissonRateInterval(bucket.EventCount, bucket.ReachableServiceDays, *confidence)
		if intervalErr != nil {
			fail(fmt.Sprintf("%s/%s: %v", key.ServiceClass, key.EventType, intervalErr))
		}
		rows = append(rows, reportRow{
			ExperimentID: *experimentID, ServiceClass: string(key.ServiceClass),
			EventType: string(key.EventType), EventCount: bucket.EventCount,
			ReachableServiceDays: bucket.ReachableServiceDays,
			EndpointCount:        bucket.EndpointCount, UniqueSourceCount: bucket.UniqueSourceCount,
			RatePerServiceDay:    rate,
			RateLowPerServiceDay: low, RateHighPerServiceDay: high,
			IntervalMethod: "garwood-poisson", Confidence: *confidence,
			ObservationStart:  bucket.ObservationStart.UTC().Format("2006-01-02T15:04:05Z07:00"),
			ObservationEnd:    bucket.ObservationEnd.UTC().Format("2006-01-02T15:04:05Z07:00"),
			DeduplicationRule: "event count is not deduplicated; uniqueSourceCount uses keyed sourceKey",
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ServiceClass == rows[j].ServiceClass {
			return rows[i].EventType < rows[j].EventType
		}
		return rows[i].ServiceClass < rows[j].ServiceClass
	})
	if err := json.NewEncoder(os.Stdout).Encode(rows); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "calibration-report:", message)
	os.Exit(2)
}

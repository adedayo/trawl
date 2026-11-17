// Command calibration-ingest accepts newline-delimited sensor events and
// stores redacted calibration evidence. It does not open a listener or retain
// the source value supplied by the sensor.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
	_ "github.com/adedayo/trawl/pkg/store/sqlite"
)

type sensorEvent struct {
	Source             string `json:"source"`
	EventType          string `json:"eventType"`
	ObservedAt         string `json:"observedAt"`
	Accepted           bool   `json:"accepted"`
	ProtocolIdentified bool   `json:"protocolIdentified"`
}

type reachabilityEvent struct {
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt"`
}

func main() {
	experimentID := flag.String("experiment", "", "calibration experiment identifier")
	endpointID := flag.String("endpoint", "", "instrumented endpoint identifier")
	serviceClass := flag.String("service-class", "", "contact service class")
	transport := flag.String("transport", "tcp", "transport protocol")
	port := flag.Uint("port", 0, "instrumented service port")
	db := flag.String("db", "", "SQLite database path")
	dryRun := flag.Bool("dry-run", false, "validate and redact input without writing to SQLite")
	reachabilityStart := flag.String("reachability-start", "", "RFC3339 start of a reachable interval")
	reachabilityEnd := flag.String("reachability-end", "", "RFC3339 end of a reachable interval")
	reachabilityFile := flag.String("reachability-file", "", "newline-delimited reachable intervals")
	flag.Parse()

	key := os.Getenv("TRAWL_CALIBRATION_KEY")
	if key == "" {
		fail("TRAWL_CALIBRATION_KEY is required")
	}
	collector, err := contact.NewCalibrationCollector(
		*experimentID, *endpointID, contact.ServiceClass(*serviceClass), *transport, uint16(*port), key)
	if err != nil {
		fail(err.Error())
	}

	var ingest *service.CalibrationIngestService
	var dbStore store.Store
	if !*dryRun {
		dbStore, err = store.Open(*db)
		if err != nil {
			fail(err.Error())
		}
		defer dbStore.Close()
		ingest = service.NewCalibrationIngestService(dbStore)
	}

	reachabilityCount := 0
	if *reachabilityFile != "" && (*reachabilityStart != "" || *reachabilityEnd != "") {
		fail("use --reachability-file or --reachability-start/--reachability-end, not both")
	}
	if (*reachabilityStart == "") != (*reachabilityEnd == "") {
		fail("set both --reachability-start and --reachability-end")
	}
	if *reachabilityStart != "" {
		startedAt, parseErr := time.Parse(time.RFC3339Nano, *reachabilityStart)
		if parseErr != nil {
			fail(fmt.Sprintf("reachability-start must be RFC3339: %v", parseErr))
		}
		endedAt, parseErr := time.Parse(time.RFC3339Nano, *reachabilityEnd)
		if parseErr != nil {
			fail(fmt.Sprintf("reachability-end must be RFC3339: %v", parseErr))
		}
		interval, intervalErr := collector.Reachability(startedAt, endedAt)
		if intervalErr != nil {
			fail(intervalErr.Error())
		}
		if !*dryRun {
			if err := ingest.SaveReachability(context.Background(), interval); err != nil {
				fail(err.Error())
			}
		}
		reachabilityCount = 1
	}
	if *reachabilityFile != "" {
		file, openErr := os.Open(*reachabilityFile)
		if openErr != nil {
			fail(fmt.Sprintf("opening reachability file: %v", openErr))
		}
		reachabilityScanner := bufio.NewScanner(file)
		reachabilityScanner.Buffer(make([]byte, 4096), 1024*1024)
		intervals := []contact.ReachabilityInterval{}
		fileLine := 0
		for reachabilityScanner.Scan() {
			fileLine++
			var input reachabilityEvent
			if err := json.Unmarshal(reachabilityScanner.Bytes(), &input); err != nil {
				file.Close()
				fail(fmt.Sprintf("reachability line %d: invalid JSON: %v", fileLine, err))
			}
			startedAt, parseErr := time.Parse(time.RFC3339Nano, input.StartedAt)
			if parseErr != nil {
				file.Close()
				fail(fmt.Sprintf("reachability line %d: startedAt must be RFC3339: %v", fileLine, parseErr))
			}
			endedAt, parseErr := time.Parse(time.RFC3339Nano, input.EndedAt)
			if parseErr != nil {
				file.Close()
				fail(fmt.Sprintf("reachability line %d: endedAt must be RFC3339: %v", fileLine, parseErr))
			}
			interval, intervalErr := collector.Reachability(startedAt, endedAt)
			if intervalErr != nil {
				file.Close()
				fail(fmt.Sprintf("reachability line %d: %v", fileLine, intervalErr))
			}
			intervals = append(intervals, interval)
		}
		if scanErr := reachabilityScanner.Err(); scanErr != nil {
			file.Close()
			fail(fmt.Sprintf("reading reachability file: %v", scanErr))
		}
		if closeErr := file.Close(); closeErr != nil {
			fail(fmt.Sprintf("closing reachability file: %v", closeErr))
		}
		if _, validationErr := contact.AggregateCalibration(nil, intervals); validationErr != nil {
			fail(fmt.Sprintf("validating reachability file: %v", validationErr))
		}
		if !*dryRun {
			for _, interval := range intervals {
				if err := ingest.SaveReachability(context.Background(), interval); err != nil {
					fail(fmt.Sprintf("saving reachability interval: %v", err))
				}
			}
		}
		reachabilityCount = len(intervals)
	}

	accepted := 0
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var input sensorEvent
		if err := json.Unmarshal(scanner.Bytes(), &input); err != nil {
			fail(fmt.Sprintf("line %d: invalid JSON: %v", line, err))
		}
		observedAt, err := time.Parse(time.RFC3339Nano, input.ObservedAt)
		if err != nil {
			fail(fmt.Sprintf("line %d: observedAt must be RFC3339: %v", line, err))
		}
		observation, err := collector.Observe(input.Source, contact.CalibrationEventType(input.EventType), observedAt, input.Accepted, input.ProtocolIdentified)
		if err != nil {
			fail(fmt.Sprintf("line %d: %v", line, err))
		}
		if !*dryRun {
			if err := ingest.SaveObservation(context.Background(), observation); err != nil {
				fail(fmt.Sprintf("line %d: %v", line, err))
			}
		}
		accepted++
	}
	if err := scanner.Err(); err != nil {
		fail(fmt.Sprintf("reading stdin: %v", err))
	}
	output := map[string]any{"status": "accepted", "observations": accepted, "reachabilityIntervals": reachabilityCount, "dryRun": *dryRun}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	message = strings.TrimSpace(message)
	fmt.Fprintln(os.Stderr, "calibration-ingest:", message)
	os.Exit(2)
}

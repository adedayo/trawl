// Command service-class-report measures how the contact taxonomy classifies
// the service observations already stored by Trawl.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"

	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/store"
	_ "github.com/adedayo/trawl/pkg/store/sqlite"
)

type classReport struct {
	TotalObservations  int            `json:"totalObservations"`
	SpecificClassCount int            `json:"specificClassCount"`
	OtherTCPCount      int            `json:"otherTcpCount"`
	SpecificClassRatio float64        `json:"specificClassRatio"`
	ClassCounts        map[string]int `json:"classCounts"`
	RawServiceLabels   map[string]int `json:"rawServiceLabels"`
}

func main() {
	db := flag.String("db", "", "SQLite database path")
	flag.Parse()

	dbStore, err := store.Open(*db)
	if err != nil {
		fail(err.Error())
	}
	defer dbStore.Close()

	observations, err := dbStore.GetServiceObservations(context.Background(), "")
	if err != nil {
		fail(err.Error())
	}
	report := classReport{
		ClassCounts:      map[string]int{},
		RawServiceLabels: map[string]int{},
	}
	for _, observation := range observations {
		serviceClass := contact.ClassifyService(observation.Service, observation.Port, observation.Protocol)
		report.TotalObservations++
		report.ClassCounts[string(serviceClass)]++
		report.RawServiceLabels[observation.Service]++
		if serviceClass == contact.ServiceClassOtherTCP {
			report.OtherTCPCount++
		} else {
			report.SpecificClassCount++
		}
	}
	if report.TotalObservations > 0 {
		report.SpecificClassRatio = float64(report.SpecificClassCount) / float64(report.TotalObservations)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	println("service-class-report:", message)
	os.Exit(2)
}

// Command feed-ingest applies one complete vulnerability catalogue snapshot.
// It supports local files for air-gapped installations and configured URLs
// for scheduled operation; both paths use the same parser and application code.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/adedayo/trawl/pkg/feed"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
	_ "github.com/adedayo/trawl/pkg/store/sqlite"
)

func main() {
	feedName := flag.String("feed", "", "feed name: cisa-kev, epss, or nvd")
	file := flag.String("file", "", "local feed file (offline mode)")
	endpoint := flag.String("url", "", "configured feed endpoint (network mode)")
	db := flag.String("db", "", "SQLite database path")
	threshold := flag.Float64("epss-threshold", 0, "EPSS threshold crossing to record, or 0 to disable")
	flag.Parse()

	if (*file == "") == (*endpoint == "") || *feedName == "" {
		fail("set --feed and exactly one of --file or --url")
	}

	name := feed.Name(*feedName)
	var snapshot *feed.Snapshot
	var err error
	if *file != "" {
		snapshot, err = feed.LoadFile(name, *file)
	} else {
		client, clientErr := feed.NewClient(nil, map[feed.Name]string{name: *endpoint})
		if clientErr != nil {
			fail(clientErr.Error())
		}
		result, fetchErr := client.Fetch(context.Background(), name)
		if fetchErr != nil {
			fail(fetchErr.Error())
		}
		if result.NotModified {
			fmt.Println(`{"status":"not_modified"}`)
			return
		}
		snapshot = result.Snapshot
	}
	if err != nil {
		fail(err.Error())
	}

	dbStore, err := store.Open(*db)
	if err != nil {
		fail(err.Error())
	}
	defer dbStore.Close()

	regressions, err := service.NewFeedIngestService(dbStore).ApplySnapshot(
		context.Background(), snapshot, *threshold)
	if err != nil {
		fail(err.Error())
	}
	output := map[string]any{
		"snapshot":    snapshot,
		"regressions": regressions,
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "feed-ingest:", message)
	os.Exit(2)
}

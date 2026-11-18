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
	dryRun := flag.Bool("dry-run", false, "validate arguments and report the planned feed action without network or database writes")
	flag.Parse()

	if (*file == "") == (*endpoint == "") || *feedName == "" {
		fail("set --feed and exactly one of --file or --url")
	}

	name := feed.Name(*feedName)
	switch name {
	case feed.KEV, feed.EPSS, feed.NVD:
	default:
		fail("unsupported feed " + *feedName + " (want cisa-kev, epss, or nvd)")
	}

	if *dryRun {
		mode := "network"
		source := *endpoint
		if *file != "" {
			mode = "offline"
			source = *file
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"dryRun": true,
			"feed":   name,
			"mode":   mode,
			"source": source,
		})
		return
	}

	dbStore, err := store.Open(*db)
	if err != nil {
		fail(err.Error())
	}
	defer dbStore.Close()

	svc := service.NewFeedIngestService(dbStore)
	var snapshot any
	var regressions []store.Regression
	if *file != "" {
		loaded, loadErr := feed.LoadFile(name, *file)
		if loadErr != nil {
			fail(loadErr.Error())
		}
		regressions, err = svc.ApplySnapshot(context.Background(), loaded, *threshold)
		snapshot = loaded
	} else {
		client, clientErr := feed.NewClient(nil, map[feed.Name]string{name: *endpoint})
		if clientErr != nil {
			fail(clientErr.Error())
		}
		regressions, err = svc.FetchAndApply(context.Background(), client, name, *threshold)
		if err == nil {
			snapshot, _ = dbStore.GetLatestFeedSnapshot(context.Background(), string(name))
		}
	}
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

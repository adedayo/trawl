package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func runServer() {
	// Initialize Store
	dbStore, err := sqlite.NewSQLiteStore("")
	if err != nil {
		log.Fatalf("failed to initialize sqlite store: %v", err)
	}
	defer dbStore.Close()

	// Initialize Event Bus
	eb := event.NewMemoryBus()

	// Handle WebSocket connections
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// In a real implementation we would upgrade to WebSocket and listen/push events.
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("WebSocket Endpoint"))
	})

	// Setup basic API Handlers
	http.HandleFunc("/api/v1/assets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			assets, err := dbStore.GetAssets(context.Background(), "")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(assets)
		}
	})

	// Start server
	log.Println("Server running on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Server failed: ", err)
	}

	// Just a demonstration that event bus is accessible
	_ = eb
}

package main

import (
	"embed"
	"log"

	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store/sqlite"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:app/dist/trawl-app/browser
var assets embed.FS

func main() {
	// Initialize SQLite Store
	s, err := sqlite.NewSQLiteStore("")
	if err != nil {
		log.Fatalf("failed to initialize sqlite store: %v", err)
	}
	defer s.Close()

	// Initialize Event Bus
	eventBus := event.NewMemoryBus()

	// Create application with options
	app := NewApp(s, eventBus)

	err = wails.Run(&options.App{
		Title:  "Trawl",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	})

	if err != nil {
		log.Fatal("wails run failed:", err)
	}
}

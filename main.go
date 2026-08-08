package main

import (
	"embed"
	"log"
	"os"

	"github.com/adedayo/trawl/config/signals"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/version"

	// The desktop build is file-backed by definition, so the SQLite backend is
	// linked in to register itself with the store factory. It is imported for
	// that effect alone; the constructor is never named here.
	_ "github.com/adedayo/trawl/pkg/store/sqlite"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:app/dist/trawl-app/browser
var assets embed.FS

func main() {
	// A desktop user cannot read a --version flag, and a support request that
	// begins "the latest one" is not a version. Recording it at startup means
	// the log a user is asked to attach already answers the question.
	build := version.Get()
	log.Printf("Trawl %s (%s)", build.Version, build.Platform)

	// Initialize the store
	//
	// An empty DSN selects the default backend at its default location, which
	// is what a desktop install wants. TRAWL_DB_DSN is honoured so that the
	// desktop app can be pointed at a shared database without a separate
	// build.
	s, err := store.Open(os.Getenv("TRAWL_DB_DSN"))
	if err != nil {
		log.Fatalf("failed to open the store: %v", err)
	}
	defer s.Close()

	// Initialize Event Bus
	eventBus := event.NewMemoryBus()

	// Create application with options
	app, err := NewApp(s, eventBus, signals.RegistryJSON())
	if err != nil {
		log.Fatalf("failed to initialize the application: %v", err)
	}

	err = wails.Run(&options.App{
		Title:  "Trawl " + build.Version,
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

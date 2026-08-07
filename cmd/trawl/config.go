package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// scanMode decides how a scan request is executed.
//
// This is not a tuning knob; it is a correctness setting that depends on
// whether the runtime keeps executing the process after a response has been
// written.
type scanMode string

const (
	// scanBackground returns 202 immediately and continues the scan on a
	// goroutine. It is correct only where the process is guaranteed CPU
	// regardless of whether a request is in flight — a long-running container
	// or the desktop application.
	scanBackground scanMode = "background"

	// scanInline performs the scan before responding.
	//
	// This is what Cloud Run requires under its default CPU allocation, where
	// an instance is throttled to near-zero CPU once it has written its final
	// response byte. A goroutine started before that point does not fail; it
	// stalls, possibly for hours, until some unrelated request happens to wake
	// the instance — or is destroyed when the instance scales to zero. The
	// caller has already been told the scan started, so the loss is silent,
	// which is the worst property a security tool can have.
	scanInline scanMode = "inline"
)

// runtimeConfig is the deployment's stated shape.
//
// Trawl runs as a desktop application, as a long-lived container, and on
// autoscaled container platforms that stop and start it on demand. Those
// differ in ways that change what is safe to do, so the differences are read
// once, stated in the log, and applied — rather than being assumed.
type runtimeConfig struct {
	addr     string
	dbDSN    string
	token    string
	scanMode scanMode

	// managed reports whether an autoscaling platform controls this process's
	// lifecycle: it may be started on demand, throttled between requests, and
	// terminated after SIGTERM with a short grace period.
	managed bool

	// platform names the detected environment, for the startup log.
	platform string

	// shutdownGrace bounds how long shutdown waits for work in flight.
	shutdownGrace time.Duration
}

// loadRuntimeConfig reads the environment and derives safe defaults.
func loadRuntimeConfig() runtimeConfig {
	cfg := runtimeConfig{
		dbDSN:         databaseDSN(),
		token:         os.Getenv("TRAWL_AUTH_TOKEN"),
		shutdownGrace: 10 * time.Second,
		platform:      "container",
	}

	// Cloud Run sets K_SERVICE on every instance, and Knative-derived
	// platforms follow the same convention. Detection rather than
	// configuration, because an operator who has to remember to set a flag is
	// an operator who will one day forget, and the failure is silent.
	if os.Getenv("K_SERVICE") != "" {
		cfg.managed = true
		cfg.platform = "cloud-run"
		// Cloud Run sends SIGTERM and allows roughly ten seconds. Asking for
		// longer than the platform grants does not extend it; it only means
		// being killed mid-wait.
		cfg.shutdownGrace = 8 * time.Second
	}

	// The platform's injected PORT is authoritative where present: an instance
	// that listens elsewhere never passes its health check, and the deployment
	// fails in a way that looks like the application being broken.
	if port := os.Getenv("PORT"); port != "" {
		cfg.addr = ":" + port
	} else if addr := os.Getenv("TRAWL_LISTEN_ADDR"); addr != "" {
		cfg.addr = addr
	} else {
		cfg.addr = ":8080"
	}

	// An explicit setting always wins; the default is derived from the
	// platform, because that is where the correctness constraint lives.
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TRAWL_SCAN_MODE"))) {
	case string(scanInline):
		cfg.scanMode = scanInline
	case string(scanBackground):
		cfg.scanMode = scanBackground
	default:
		if cfg.managed {
			cfg.scanMode = scanInline
		} else {
			cfg.scanMode = scanBackground
		}
	}

	return cfg
}

// databaseDSN resolves which storage backend this deployment wants.
//
// TRAWL_DB_DSN is scheme-qualified and selects a backend; TRAWL_DB_PATH is the
// older, SQLite-only spelling and is still honoured so that existing
// deployments keep working. A bare path is a valid DSN — the store's factory
// treats an unqualified value as SQLite — so no translation is needed here.
func databaseDSN() string {
	if dsn := strings.TrimSpace(os.Getenv("TRAWL_DB_DSN")); dsn != "" {
		return dsn
	}
	return strings.TrimSpace(os.Getenv("TRAWL_DB_PATH"))
}

// announce states the operating assumptions at startup.
//
// Each of these is a condition under which Trawl would otherwise mislead: a
// scan that never runs, a database that vanishes, an unauthenticated ingest
// endpoint. Stating them in the log costs nothing and makes the deployment's
// behaviour legible to whoever is reading it at three in the morning.
func (cfg runtimeConfig) announce() {
	log.Printf("Trawl server starting: platform=%s addr=%s scan-mode=%s store=%s",
		cfg.platform, cfg.addr, cfg.scanMode, store.SchemeOf(cfg.dbDSN))

	if cfg.token == "" {
		log.Println("WARNING: TRAWL_AUTH_TOKEN is unset — mutating and ingest endpoints are unauthenticated")
	}

	if cfg.scanMode == scanInline {
		log.Println("Scans run inline: the request is held until the scan completes, because " +
			"this platform does not guarantee CPU to a process that has finished responding.")
	}

	if !cfg.managed {
		return
	}

	// The single-writer warning applies to a file-backed store only. A
	// networked backend is precisely the fix for it, so repeating the warning
	// there would teach operators to ignore it.
	switch store.SchemeOf(cfg.dbDSN) {
	case sqlite.SchemeSQLite, sqlite.SchemeFile:
	default:
		return
	}

	// SQLite on an autoscaled platform is the failure worth being loud about.
	// Each instance gets a private filesystem, so two instances mean two
	// databases and a dashboard whose answer depends on which one served the
	// request. Scaling to zero then discards whichever it was.
	log.Println("NOTICE: state is held in SQLite on this instance's filesystem. On an " +
		"autoscaling platform this is only coherent with max-instances=1 and a mounted, " +
		"durable volume. Without both, findings will diverge between instances and be " +
		"discarded when an instance is reclaimed.")
}

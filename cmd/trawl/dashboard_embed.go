//go:build embed_dashboard

package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

// dashboardFiles is the built Angular bundle, embedded when the
// `embed_dashboard` build tag is set.
//
// Serving the dashboard from the Go binary is what lets Trawl run as a single
// container on platforms that serve one container per service. It also removes
// a class of bug outright: with the UI and the API in the same process and the
// same origin, there is no proxy to configure, no CORS to get wrong, and no
// possibility of the dashboard being a different version from the API it calls.
//
//go:embed all:webroot
var dashboardFiles embed.FS

// mountDashboard serves the bundle with SPA fallback.
func (s *server) mountDashboard(mux *http.ServeMux) {
	root, err := fs.Sub(dashboardFiles, "webroot")
	if err != nil {
		log.Printf("WARNING: the embedded dashboard could not be mounted: %v", err)
		return
	}

	files := http.FileServer(http.FS(root))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// An API path that reached here matched no route, and must not be
		// answered with index.html. Returning the SPA shell for an unknown
		// endpoint gives the caller HTTP 200 and a page of HTML where they
		// expected JSON, which surfaces as a parse error far from its cause.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Anything without a file extension is a client-side route: hand it
		// the shell and let the router resolve it.
		if path := strings.TrimPrefix(r.URL.Path, "/"); path != "" {
			if _, err := fs.Stat(root, path); err != nil {
				r = r.Clone(r.Context())
				r.URL.Path = "/"
			}
		}
		files.ServeHTTP(w, r)
	})

	log.Println("Dashboard mounted at / from the embedded bundle.")
}

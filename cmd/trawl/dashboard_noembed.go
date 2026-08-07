//go:build !embed_dashboard

package main

import "net/http"

// mountDashboard is a no-op in builds without the embedded frontend.
//
// The compose deployment serves the dashboard from a separate nginx container,
// and the desktop build has its own. Only the single-container deployment
// needs the bundle inside the binary, so only it pays the build cost of
// producing one.
func (s *server) mountDashboard(mux *http.ServeMux) {}

// Package api exposes the monitored devices' state over a small,
// dependency-free REST API built on net/http.
//
// Endpoints:
//
//	GET /api/v1/devices              -> list of device names with data
//	GET /api/v1/devices/{name}       -> latest snapshot for a device
//	GET /api/v1/devices/{name}/history -> retained snapshot history
//	GET /healthz                     -> liveness check
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/freeb5d/whatunga/internal/store"
)

// Server wraps the shared History store with HTTP handlers.
type Server struct {
	history *store.History
	mux     *http.ServeMux
}

// NewServer builds a Server backed by the given History store and
// registers all routes.
func NewServer(history *store.History) *Server {
	s := &Server{history: history, mux: http.NewServeMux()}
	s.routes()
	return s
}

// ServeHTTP lets Server itself be used as an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/api/v1/devices", s.handleListDevices)
	s.mux.HandleFunc("/api/v1/devices/", s.handleDeviceRoute)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"devices": s.history.Devices(),
	})
}

// handleDeviceRoute dispatches "/api/v1/devices/{name}" and
// "/api/v1/devices/{name}/history" — kept as one handler with manual
// path parsing to avoid pulling in a router dependency for two routes.
func (s *Server) handleDeviceRoute(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device name is required"})
		return
	}
	device := parts[0]

	if len(parts) == 2 && parts[1] == "history" {
		s.handleDeviceHistory(w, device)
		return
	}
	if len(parts) == 1 {
		s.handleDeviceLatest(w, device)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (s *Server) handleDeviceLatest(w http.ResponseWriter, device string) {
	snap, ok := s.history.Latest(device)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown device or no data yet: " + device})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleDeviceHistory(w http.ResponseWriter, device string) {
	snaps := s.history.All(device)
	if len(snaps) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown device or no data yet: " + device})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device":    device,
		"snapshots": snaps,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

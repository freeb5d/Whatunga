package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freeb5d/whatunga/internal/monitor"
	"github.com/freeb5d/whatunga/internal/store"
)

func TestHandleHealth(t *testing.T) {
	server := NewServer(store.NewHistory(10))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleDeviceLatest_NotFound(t *testing.T) {
	server := NewServer(store.NewHistory(10))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/nope", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeviceLatest_ReturnsMostRecentSnapshot(t *testing.T) {
	history := store.NewHistory(10)
	history.Add(monitor.Snapshot{Device: "router1", System: monitor.SystemStatus{CPULoad: 5}})
	history.Add(monitor.Snapshot{Device: "router1", System: monitor.SystemStatus{CPULoad: 42}})

	server := NewServer(history)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/router1", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var snap monitor.Snapshot
	if err := json.NewDecoder(rec.Body).Decode(&snap); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if snap.System.CPULoad != 42 {
		t.Errorf("CPULoad = %d, want 42 (the most recent snapshot)", snap.System.CPULoad)
	}
}

func TestHandleDeviceHistory_ReturnsAllSnapshots(t *testing.T) {
	history := store.NewHistory(10)
	history.Add(monitor.Snapshot{Device: "router1", System: monitor.SystemStatus{CPULoad: 5}})
	history.Add(monitor.Snapshot{Device: "router1", System: monitor.SystemStatus{CPULoad: 42}})

	server := NewServer(history)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/router1/history", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Device    string             `json:"device"`
		Snapshots []monitor.Snapshot `json:"snapshots"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(body.Snapshots) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(body.Snapshots))
	}
}

func TestHandleListDevices(t *testing.T) {
	history := store.NewHistory(10)
	history.Add(monitor.Snapshot{Device: "router1"})
	history.Add(monitor.Snapshot{Device: "router2"})

	server := NewServer(history)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	var body struct {
		Devices []string `json:"devices"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(body.Devices) != 2 {
		t.Fatalf("got %d devices, want 2", len(body.Devices))
	}
}

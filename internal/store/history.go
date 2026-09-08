// Package store keeps a bounded, in-memory history of recent snapshots
// for each monitored device, so the REST API can serve "last N polls"
// without needing an external database for what is meant to be a
// lightweight monitoring tool.
package store

import (
	"sync"

	"github.com/freeb5d/whatunga/internal/monitor"
)

// History holds a fixed-capacity ring buffer of snapshots per device
// name. It is safe for concurrent use — the poller goroutine writes
// while HTTP handlers read.
type History struct {
	mu       sync.RWMutex
	capacity int
	byDevice map[string][]monitor.Snapshot
}

// NewHistory returns a History that keeps up to capacity snapshots per
// device. Older snapshots are dropped once capacity is exceeded.
func NewHistory(capacity int) *History {
	if capacity < 1 {
		capacity = 1
	}
	return &History{
		capacity: capacity,
		byDevice: make(map[string][]monitor.Snapshot),
	}
}

// Add appends a new snapshot for its device, evicting the oldest one
// if the device's history is already at capacity.
func (h *History) Add(snap monitor.Snapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()

	entries := h.byDevice[snap.Device]
	entries = append(entries, snap)
	if len(entries) > h.capacity {
		entries = entries[len(entries)-h.capacity:]
	}
	h.byDevice[snap.Device] = entries
}

// Latest returns the most recent snapshot for a device, and whether
// one exists yet.
func (h *History) Latest(device string) (monitor.Snapshot, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries := h.byDevice[device]
	if len(entries) == 0 {
		return monitor.Snapshot{}, false
	}
	return entries[len(entries)-1], true
}

// All returns a copy of the full retained history for a device,
// oldest first.
func (h *History) All(device string) []monitor.Snapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries := h.byDevice[device]
	out := make([]monitor.Snapshot, len(entries))
	copy(out, entries)
	return out
}

// Devices returns the names of all devices with at least one recorded
// snapshot.
func (h *History) Devices() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	names := make([]string, 0, len(h.byDevice))
	for name := range h.byDevice {
		names = append(names, name)
	}
	return names
}

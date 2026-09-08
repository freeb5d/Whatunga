package store

import (
	"testing"

	"github.com/freeb5d/whatunga/internal/monitor"
)

func TestHistory_AddAndLatest(t *testing.T) {
	h := NewHistory(3)

	_, ok := h.Latest("router1")
	if ok {
		t.Fatalf("expected no snapshot before any Add")
	}

	h.Add(monitor.Snapshot{Device: "router1", System: monitor.SystemStatus{CPULoad: 10}})
	h.Add(monitor.Snapshot{Device: "router1", System: monitor.SystemStatus{CPULoad: 20}})

	latest, ok := h.Latest("router1")
	if !ok {
		t.Fatalf("expected a snapshot after Add")
	}
	if latest.System.CPULoad != 20 {
		t.Errorf("CPULoad = %d, want 20", latest.System.CPULoad)
	}
}

func TestHistory_EvictsOldestBeyondCapacity(t *testing.T) {
	h := NewHistory(2)

	h.Add(monitor.Snapshot{Device: "router1", System: monitor.SystemStatus{CPULoad: 1}})
	h.Add(monitor.Snapshot{Device: "router1", System: monitor.SystemStatus{CPULoad: 2}})
	h.Add(monitor.Snapshot{Device: "router1", System: monitor.SystemStatus{CPULoad: 3}})

	all := h.All("router1")
	if len(all) != 2 {
		t.Fatalf("got %d entries, want 2 (capacity)", len(all))
	}
	if all[0].System.CPULoad != 2 || all[1].System.CPULoad != 3 {
		t.Errorf("expected the oldest entry (CPULoad=1) to be evicted, got %+v", all)
	}
}

func TestHistory_DevicesAreIndependent(t *testing.T) {
	h := NewHistory(5)

	h.Add(monitor.Snapshot{Device: "router1"})
	h.Add(monitor.Snapshot{Device: "router2"})

	devices := h.Devices()
	if len(devices) != 2 {
		t.Fatalf("got %d devices, want 2: %v", len(devices), devices)
	}
}

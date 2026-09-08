package monitor

import "time"

// SystemStatus mirrors the useful fields from /system/resource/print.
type SystemStatus struct {
	Uptime       string    `json:"uptime"`
	Version      string    `json:"version"`
	BoardName    string    `json:"board_name"`
	CPULoad      int       `json:"cpu_load_percent"`
	FreeMemoryKB int64     `json:"free_memory_kb"`
	TotalMemKB   int64     `json:"total_memory_kb"`
	CheckedAt    time.Time `json:"checked_at"`
}

// Interface mirrors the useful fields from /interface/print.
type Interface struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Running  bool   `json:"running"`
	Disabled bool   `json:"disabled"`
	RxBytes  int64  `json:"rx_bytes"`
	TxBytes  int64  `json:"tx_bytes"`
}

// Snapshot is one point-in-time poll of a device: its system status plus
// all interfaces. Snapshots are what gets kept in the history store and
// served over the REST API.
type Snapshot struct {
	Device     string      `json:"device"`
	System     SystemStatus `json:"system"`
	Interfaces []Interface `json:"interfaces"`
}

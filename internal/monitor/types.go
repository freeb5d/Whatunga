package monitor

import "time"

// Kind identifies which protocol/backend produced a Snapshot, so the
// web UI and REST API can label devices correctly and callers can
// avoid assuming fields that a given device type simply can't report
// (e.g. generic SNMP firewalls rarely expose a CPU load OID).
type Kind string

const (
	KindRouterOS      Kind = "routeros"
	KindILO           Kind = "ilo"
	KindWindowsServer Kind = "windows"
	KindSNMPFirewall  Kind = "snmp-firewall"
)

// SystemStatus is a best-effort common shape across every device kind
// Whatunga supports. Not every field is meaningful for every kind —
// e.g. generic SNMP devices rarely expose CPU load through a
// standard MIB, and iLO's Redfish API reports health/power state
// rather than a live CPU percentage. Fields that don't apply for a
// given kind are simply left at their zero value; the web UI renders
// those as "—" rather than a misleading 0.
type SystemStatus struct {
	Uptime       string    `json:"uptime"`
	Version      string    `json:"version"`       // firmware/OS/RouterOS version
	BoardName    string    `json:"board_name"`    // model/board/product name
	Health       string    `json:"health"`        // e.g. Redfish "OK"/"Warning"/"Critical" (iLO)
	PowerState   string    `json:"power_state"`   // "On"/"Off" (iLO); empty where not applicable
	CPULoad      int       `json:"cpu_load_percent"`
	CPULoadKnown bool      `json:"cpu_load_known"` // false when this device kind can't report CPU load
	FreeMemoryKB int64     `json:"free_memory_kb"`
	TotalMemKB   int64     `json:"total_memory_kb"`
	CheckedAt    time.Time `json:"checked_at"`
}

// Interface is a best-effort common shape for a network interface,
// used across RouterOS, Windows, and SNMP-polled firewalls. (iLO
// snapshots normally have none — server health monitoring, not
// network interface monitoring — so Interfaces will just be empty.)
type Interface struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Running  bool   `json:"running"`
	Disabled bool   `json:"disabled"`
	RxBytes  int64  `json:"rx_bytes"`
	TxBytes  int64  `json:"tx_bytes"`
}

// Snapshot is one point-in-time poll of a device: its system status
// plus all interfaces (if any). Snapshots are what gets kept in the
// history store and served over the REST API and web UI.
type Snapshot struct {
	Device     string      `json:"device"`
	Kind       Kind        `json:"kind"`
	System     SystemStatus `json:"system"`
	Interfaces []Interface `json:"interfaces"`
}

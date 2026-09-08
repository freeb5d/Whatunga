// Package monitor defines the common Poller interface that every
// supported device kind (RouterOS, HPE iLO, Windows Server, and
// generic SNMP-polled firewalls) implements, plus the shared,
// best-effort Snapshot/SystemStatus/Interface shapes they all
// populate. See routeros_poller.go, ilo_poller.go, winrm_poller.go,
// and snmp_poller.go for the per-kind implementations.
package monitor

// Poller polls one device on demand and can be closed when no longer
// needed. Every device-kind poller in this package (RouterOSPoller,
// ILOPoller, WindowsPoller, SNMPFirewallPoller) implements this.
type Poller interface {
	// Snapshot takes a fresh, point-in-time reading of the device.
	Snapshot() (Snapshot, error)
	// Close releases any held connection/resources.
	Close() error
}

// This file implements the Poller interface (see poller.go) for
// any device that speaks standard SNMPv2c — in practice, this
// covers most firewall models (pfSense, FortiGate, Cisco ASA,
// Sophos XG, and more) using nothing vendor-specific, just the
// standard MIB-II interface table and system group that every
// SNMP agent implements.
//
// CPU load is deliberately not reported here: there is no
// standardized MIB-II object for it, and vendor-specific OIDs
// (e.g. Cisco's CPMCPUTotal5minRev, FortiGate's fgSysCpuUsage)
// would defeat the point of a single vendor-neutral implementation.
// SystemStatus.CPULoadKnown is always false for this poller kind.
package monitor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/freeb5d/whatunga/internal/snmp"
)

// Standard MIB-II OIDs used by SNMPFirewallPoller.
const (
	oidSysDescr     = "1.3.6.1.2.1.1.1.0"
	oidSysUpTime    = "1.3.6.1.2.1.1.3.0"
	oidIfDescr      = "1.3.6.1.2.1.2.2.1.2"
	oidIfOperStatus = "1.3.6.1.2.1.2.2.1.8"
	oidIfInOctets   = "1.3.6.1.2.1.2.2.1.10"
	oidIfOutOctets  = "1.3.6.1.2.1.2.2.1.16"
)

// SNMPFirewallPoller polls one SNMPv2c-capable device — named for
// its primary intended use (firewalls) but equally usable against
// any device exposing the standard MIB-II interface table (a
// switch, a Windows Server with the SNMP service enabled, etc.).
type SNMPFirewallPoller struct {
	device string
	client *snmp.Client
}

// NewSNMPFirewallPoller returns a Poller for the SNMP agent at
// address ("host:161" or just "host") using the given community
// string (commonly "public" for read-only access, but any site
// should set a non-default community in production).
func NewSNMPFirewallPoller(deviceName, address, community string, timeout time.Duration) *SNMPFirewallPoller {
	return &SNMPFirewallPoller{
		device: deviceName,
		client: snmp.NewClient(address, community, timeout),
	}
}

// Close is a no-op for SNMPFirewallPoller — each request is a
// fresh, connectionless UDP exchange.
func (p *SNMPFirewallPoller) Close() error { return nil }

// Snapshot fetches sysDescr/sysUpTime and walks the interface table,
// mapping the results onto Whatunga's common Snapshot shape.
func (p *SNMPFirewallPoller) Snapshot() (Snapshot, error) {
	system, err := p.systemStatus()
	if err != nil {
		return Snapshot{}, fmt.Errorf("monitor: snmp: system status: %w", err)
	}

	interfaces, err := p.interfaces()
	if err != nil {
		return Snapshot{}, fmt.Errorf("monitor: snmp: interfaces: %w", err)
	}

	return Snapshot{
		Device:     p.device,
		Kind:       KindSNMPFirewall,
		System:     system,
		Interfaces: interfaces,
	}, nil
}

func (p *SNMPFirewallPoller) systemStatus() (SystemStatus, error) {
	results, err := p.client.Get(oidSysDescr, oidSysUpTime)
	if err != nil {
		return SystemStatus{}, err
	}

	var uptimeTicks int64
	if v, ok := results[oidSysUpTime]; ok {
		uptimeTicks = v.Int
	}

	return SystemStatus{
		Version:      results[oidSysDescr].Str,
		CPULoadKnown: false, // no vendor-neutral MIB-II object for CPU load exists
		Uptime:       formatUptime(uptimeTicks / 100), // sysUpTime is in hundredths of a second
		CheckedAt:    time.Now().UTC(),
	}, nil
}

func (p *SNMPFirewallPoller) interfaces() ([]Interface, error) {
	names, err := p.client.Walk(oidIfDescr)
	if err != nil {
		return nil, err
	}
	statuses, err := p.client.Walk(oidIfOperStatus)
	if err != nil {
		return nil, err
	}
	inOctets, err := p.client.Walk(oidIfInOctets)
	if err != nil {
		return nil, err
	}
	outOctets, err := p.client.Walk(oidIfOutOctets)
	if err != nil {
		return nil, err
	}

	interfaces := make([]Interface, 0, len(names))
	for oid, nameValue := range names {
		index := ifTableIndex(oid, oidIfDescr)

		interfaces = append(interfaces, Interface{
			Name:    nameValue.Str,
			Type:    "ethernet",
			Running: statuses[oidIfOperStatus+"."+index].Int == 1, // ifOperStatus: 1 = up
			RxBytes: inOctets[oidIfInOctets+"."+index].Int,
			TxBytes: outOctets[oidIfOutOctets+"."+index].Int,
		})
	}

	return interfaces, nil
}

// ifTableIndex extracts the trailing row index from an interface
// table OID, e.g. ifTableIndex("1.3.6.1.2.1.2.2.1.2.3", oidIfDescr)
// returns "3".
func ifTableIndex(oid, columnBase string) string {
	suffix := strings.TrimPrefix(oid, columnBase+".")
	if _, err := strconv.Atoi(suffix); err != nil {
		return suffix // unexpected shape; leave as-is rather than silently dropping data
	}
	return suffix
}

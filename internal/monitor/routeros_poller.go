// This file implements the Poller interface (see poller.go) for
// MikroTik RouterOS devices, on top of the hand-written binary API
// client in internal/routeros.
package monitor

import (
	"fmt"
	"strconv"
	"time"

	"github.com/freeb5d/whatunga/internal/routeros"
)

// RouterOSPoller polls one RouterOS device on demand. It owns the
// underlying API connection for the lifetime of the Poller.
type RouterOSPoller struct {
	device string
	client *routeros.Client
}

// NewRouterOSPoller connects to a device and returns a Poller ready
// to take snapshots. The caller is responsible for calling Close
// when done.
func NewRouterOSPoller(deviceName, address, username, password string, timeout time.Duration) (*RouterOSPoller, error) {
	client, err := routeros.Dial(address, username, password, timeout)
	if err != nil {
		return nil, err
	}
	return &RouterOSPoller{device: deviceName, client: client}, nil
}

// Close closes the underlying RouterOS API connection.
func (p *RouterOSPoller) Close() error {
	return p.client.Close()
}

// Snapshot polls /system/resource/print and /interface/print and
// returns a combined, typed snapshot of the device's current state.
func (p *RouterOSPoller) Snapshot() (Snapshot, error) {
	system, err := p.systemStatus()
	if err != nil {
		return Snapshot{}, fmt.Errorf("monitor: system status: %w", err)
	}

	interfaces, err := p.interfaces()
	if err != nil {
		return Snapshot{}, fmt.Errorf("monitor: interfaces: %w", err)
	}

	return Snapshot{
		Device:     p.device,
		Kind:       KindRouterOS,
		System:     system,
		Interfaces: interfaces,
	}, nil
}

func (p *RouterOSPoller) systemStatus() (SystemStatus, error) {
	reply, err := p.client.Run("/system/resource/print")
	if err != nil {
		return SystemStatus{}, err
	}
	if reply.Status != "done" || len(reply.Rows) == 0 {
		return SystemStatus{}, fmt.Errorf("unexpected reply: %s", reply.Status)
	}

	row := reply.Rows[0]

	return SystemStatus{
		Uptime:       row["uptime"],
		Version:      row["version"],
		BoardName:    row["board-name"],
		CPULoad:      atoiOr(row["cpu-load"], 0),
		CPULoadKnown: true,
		FreeMemoryKB: atoi64Or(row["free-memory"], 0) / 1024,
		TotalMemKB:   atoi64Or(row["total-memory"], 0) / 1024,
		CheckedAt:    time.Now().UTC(),
	}, nil
}

func (p *RouterOSPoller) interfaces() ([]Interface, error) {
	reply, err := p.client.Run("/interface/print")
	if err != nil {
		return nil, err
	}
	if reply.Status != "done" {
		return nil, fmt.Errorf("unexpected reply: %s", reply.Status)
	}

	interfaces := make([]Interface, 0, len(reply.Rows))
	for _, row := range reply.Rows {
		interfaces = append(interfaces, Interface{
			Name:     row["name"],
			Type:     row["type"],
			Running:  row["running"] == "true",
			Disabled: row["disabled"] == "true",
			RxBytes:  atoi64Or(row["rx-byte"], 0),
			TxBytes:  atoi64Or(row["tx-byte"], 0),
		})
	}

	return interfaces, nil
}

// atoiOr parses s as an int, returning fallback if s is empty or invalid.
// RouterOS replies are all strings on the wire, so every numeric field
// needs this kind of best-effort conversion rather than a hard failure —
// a single unexpected field (e.g. on an unusual RouterBOARD model)
// shouldn't take down the whole poll.
func atoiOr(s string, fallback int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return fallback
}

func atoi64Or(s string, fallback int64) int64 {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	return fallback
}

// This file implements the Poller interface (see poller.go) for
// Windows Server, using WinRM to run a small PowerShell script
// remotely and parse its output. This needs no agent installed on
// the target — only WinRM enabled (`Enable-PSRemoting` on the
// server) and an administrator account to authenticate with.
//
// Unlike RouterOS's binary API, WinRM is a well-established SOAP-ish
// protocol with real complexity (WS-Management, message signing
// options, etc.) that isn't worth re-implementing by hand for a
// monitoring tool — this uses the well-known masterzen/winrm client
// library rather than reinventing it.
package monitor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/masterzen/winrm"
)

// WindowsPoller polls one Windows Server host over WinRM.
type WindowsPoller struct {
	device   string
	client   *winrm.Client
}

// NewWindowsPoller returns a Poller for a Windows Server host at
// address (host, or host:port — default WinRM ports are 5985 for
// HTTP and 5986 for HTTPS). useHTTPS selects the transport/port
// default when address has no explicit port.
func NewWindowsPoller(deviceName, address, username, password string, useHTTPS bool, insecureSkipVerify bool, timeout time.Duration) (*WindowsPoller, error) {
	host, port := splitHostPort(address, useHTTPS)

	endpoint := winrm.NewEndpoint(host, port, useHTTPS, insecureSkipVerify, nil, nil, nil, timeout)
	client, err := winrm.NewClient(endpoint, username, password)
	if err != nil {
		return nil, fmt.Errorf("monitor: windows: creating WinRM client: %w", err)
	}

	return &WindowsPoller{device: deviceName, client: client}, nil
}

// Close is a no-op for WindowsPoller — WinRM sessions in this client
// library are created per-command, there's no persistent connection
// held between Snapshot calls.
func (p *WindowsPoller) Close() error { return nil }

// statusScript prints one CSV line: CPULoad,FreeMemKB,TotalMemKB,
// UptimeSeconds,OSVersion,ComputerModel — everything Snapshot needs
// in a single remote call, to keep polling cheap.
const statusScript = `
$cpu = (Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average
$os = Get-CimInstance Win32_OperatingSystem
$cs = Get-CimInstance Win32_ComputerSystem
$uptimeSeconds = [int]((Get-Date) - $os.LastBootUpTime).TotalSeconds
Write-Output "$cpu,$($os.FreePhysicalMemory),$($os.TotalVisibleMemorySize),$uptimeSeconds,$($os.Version),$($cs.Model)"
`

// interfacesScript prints one CSV line per adapter:
// Name,Running,RxBytes,TxBytes
const interfacesScript = `
Get-NetAdapterStatistics | ForEach-Object {
	$adapter = Get-NetAdapter -InterfaceIndex $_.InterfaceIndex
	Write-Output "$($adapter.Name),$($adapter.Status -eq 'Up'),$($_.ReceivedBytes),$($_.SentBytes)"
}
`

// Snapshot runs statusScript and interfacesScript over WinRM and
// parses their CSV output into Whatunga's common Snapshot shape.
func (p *WindowsPoller) Snapshot() (Snapshot, error) {
	system, err := p.systemStatus()
	if err != nil {
		return Snapshot{}, fmt.Errorf("monitor: windows: system status: %w", err)
	}

	interfaces, err := p.interfaces()
	if err != nil {
		return Snapshot{}, fmt.Errorf("monitor: windows: interfaces: %w", err)
	}

	return Snapshot{
		Device:     p.device,
		Kind:       KindWindowsServer,
		System:     system,
		Interfaces: interfaces,
	}, nil
}

func (p *WindowsPoller) systemStatus() (SystemStatus, error) {
	out, err := p.runPowerShell(statusScript)
	if err != nil {
		return SystemStatus{}, err
	}

	fields := strings.SplitN(strings.TrimSpace(out), ",", 6)
	if len(fields) != 6 {
		return SystemStatus{}, fmt.Errorf("unexpected script output: %q", out)
	}

	freeKB := atoi64Or(fields[1], 0)
	totalKB := atoi64Or(fields[2], 0)
	uptimeSeconds := atoi64Or(fields[3], 0)

	return SystemStatus{
		Uptime:       formatUptime(uptimeSeconds),
		Version:      fields[4],
		BoardName:    fields[5],
		CPULoad:      atoiOr(fields[0], 0),
		CPULoadKnown: fields[0] != "",
		FreeMemoryKB: freeKB,
		TotalMemKB:   totalKB,
		CheckedAt:    time.Now().UTC(),
	}, nil
}

func (p *WindowsPoller) interfaces() ([]Interface, error) {
	out, err := p.runPowerShell(interfacesScript)
	if err != nil {
		return nil, err
	}

	var interfaces []Interface
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, ",", 4)
		if len(fields) != 4 {
			continue
		}
		interfaces = append(interfaces, Interface{
			Name:    fields[0],
			Type:    "ethernet",
			Running: fields[1] == "True",
			RxBytes: atoi64Or(fields[2], 0),
			TxBytes: atoi64Or(fields[3], 0),
		})
	}

	return interfaces, nil
}

func (p *WindowsPoller) runPowerShell(script string) (string, error) {
	stdout, stderr, exitCode, err := p.client.RunWithString(winrm.Powershell(script), "")
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		return "", fmt.Errorf("powershell exited %d: %s", exitCode, strings.TrimSpace(stderr))
	}
	return stdout, nil
}

// splitHostPort separates "host" or "host:port" into a host and a
// port, defaulting to WinRM's standard HTTP (5985) or HTTPS (5986)
// port when none is given explicitly.
func splitHostPort(address string, useHTTPS bool) (string, int) {
	if host, portStr, ok := strings.Cut(address, ":"); ok {
		if port, err := strconv.Atoi(portStr); err == nil {
			return host, port
		}
	}
	if useHTTPS {
		return address, 5986
	}
	return address, 5985
}

// formatUptime renders a second count as "Nd Nh Nm", matching the
// style RouterOS's own uptime string uses closely enough for the
// dashboard to show both device kinds consistently.
func formatUptime(totalSeconds int64) string {
	if totalSeconds <= 0 {
		return ""
	}
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
}

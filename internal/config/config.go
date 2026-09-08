// Package config loads Whatunga's device list, notifier list, and
// global settings from a simple config file. The format is
// intentionally minimal (not full YAML) so the whole tool stays
// dependency-free — see config.example.yaml for the exact syntax
// this parser expects.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DeviceType identifies which protocol/poller a device entry should
// use. This mirrors monitor.Kind but is kept as a separate string
// type here so the config package doesn't need to import monitor —
// the mapping from string to monitor.Kind/Poller happens in main.go.
type DeviceType string

const (
	TypeRouterOS     DeviceType = "routeros"
	TypeILO          DeviceType = "ilo"
	TypeWindows      DeviceType = "windows"
	TypeSNMPFirewall DeviceType = "snmp-firewall"
)

// Device describes one device to poll. Which fields matter depends
// on Type:
//
//   - routeros:      Address ("host:8728"), Username, Password
//   - ilo:           Address ("host" or "host:443"), Username, Password, Insecure
//   - windows:       Address ("host" or "host:5985"/"host:5986"), Username, Password, UseHTTPS, Insecure
//   - snmp-firewall: Address ("host" or "host:161"), Community
type Device struct {
	Name      string
	Type      DeviceType
	Address   string
	Username  string
	Password  string
	Community string // snmp-firewall only; defaults to "public" if empty
	UseHTTPS  bool   // windows only; selects WinRM transport/default port
	Insecure  bool   // ilo/windows only; skip TLS certificate verification
}

// NotifierType identifies which Notifier implementation a notifier
// entry configures.
type NotifierType string

const (
	NotifierTelegram NotifierType = "telegram"
	NotifierEmail    NotifierType = "email"
	NotifierWebhook  NotifierType = "webhook"
)

// Notifier describes one alert destination. Which fields matter
// depends on Type:
//
//   - telegram: BotToken, ChatID
//   - email:    SMTPServer, SMTPPort, Username, Password, From, To
//   - webhook:  URL
type Notifier struct {
	Type NotifierType

	// telegram
	BotToken string
	ChatID   string

	// email
	SMTPServer string
	SMTPPort   int
	Username   string
	Password   string
	From       string
	To         []string

	// webhook
	URL string
}

// Config is the full set of devices, notifiers, and global settings.
type Config struct {
	PollInterval    time.Duration
	HistorySize     int
	ListenAddr      string // REST API (JSON) listen address
	WebAddr         string // browser admin panel listen address
	SQLitePath      string // path to the SQLite database file (users, settings)
	NotifyThreshold int    // consecutive failed polls before an alert fires
	Devices         []Device
	Notifiers       []Notifier
}

// Load reads and parses a config file. The expected format is a
// restricted subset of YAML:
//
//	poll_interval: 30s
//	history_size: 120
//	listen_addr: ":8080"
//	web_addr: ":8081"
//	sqlite_path: whatunga.db
//	notify_threshold: 3
//
//	devices:
//	  - name: office-router
//	    type: routeros
//	    address: 192.168.88.1:8728
//	    username: admin
//	    password: secret
//	  - name: web-server-ilo
//	    type: ilo
//	    address: 10.0.0.20
//	    username: administrator
//	    password: secret
//	    insecure: true
//	  - name: file-server
//	    type: windows
//	    address: 10.0.0.30
//	    username: Administrator
//	    password: secret
//	  - name: edge-firewall
//	    type: snmp-firewall
//	    address: 10.0.0.1:161
//	    community: public
//
//	notifiers:
//	  - type: telegram
//	    bot_token: 123456:ABC-DEF
//	    chat_id: "-100123456789"
//	  - type: email
//	    smtp_server: smtp.example.com
//	    smtp_port: 587
//	    username: alerts@example.com
//	    password: secret
//	    from: alerts@example.com
//	    to: ops@example.com, oncall@example.com
//	  - type: webhook
//	    url: https://example.com/hooks/whatunga
//
// "type" on a device defaults to "routeros" when omitted, for
// backward compatibility with config files written before other
// device types existed. "notify_threshold" defaults to 3 when
// omitted.
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: opening %s: %w", path, err)
	}
	defer file.Close()

	cfg := Config{
		PollInterval:    30 * time.Second,
		HistorySize:     120,
		ListenAddr:      ":8080",
		WebAddr:         ":8081",
		SQLitePath:      "whatunga.db",
		NotifyThreshold: 3,
	}

	// section tracks which top-level list ("devices" or "notifiers")
	// subsequent "- ..." entries and their indented attribute lines
	// belong to — needed because both lists reuse attribute names
	// like "username:" and "password:".
	var section string
	var currentDevice *Device
	var currentNotifier *Notifier

	flushDevice := func() {
		if currentDevice != nil {
			applyDeviceDefaults(currentDevice)
			cfg.Devices = append(cfg.Devices, *currentDevice)
			currentDevice = nil
		}
	}
	flushNotifier := func() {
		if currentNotifier != nil {
			cfg.Notifiers = append(cfg.Notifiers, *currentNotifier)
			currentNotifier = nil
		}
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		switch {
		case trimmed == "devices:":
			flushDevice()
			flushNotifier()
			section = "devices"

		case trimmed == "notifiers:":
			flushDevice()
			flushNotifier()
			section = "notifiers"

		case strings.HasPrefix(trimmed, "- name:") && section == "devices":
			flushDevice()
			currentDevice = &Device{Name: valueAfterColon(trimmed), Type: TypeRouterOS}

		case strings.HasPrefix(trimmed, "- type:") && section == "notifiers":
			flushNotifier()
			currentNotifier = &Notifier{Type: NotifierType(valueAfterColon(trimmed))}

		case strings.HasPrefix(trimmed, "type:") && section == "devices" && currentDevice != nil:
			currentDevice.Type = DeviceType(valueAfterColon(trimmed))

		case strings.HasPrefix(trimmed, "address:") && section == "devices" && currentDevice != nil:
			currentDevice.Address = valueAfterColon(trimmed)

		case strings.HasPrefix(trimmed, "username:") && section == "devices" && currentDevice != nil:
			currentDevice.Username = valueAfterColon(trimmed)

		case strings.HasPrefix(trimmed, "password:") && section == "devices" && currentDevice != nil:
			currentDevice.Password = valueAfterColon(trimmed)

		case strings.HasPrefix(trimmed, "community:") && section == "devices" && currentDevice != nil:
			currentDevice.Community = valueAfterColon(trimmed)

		case strings.HasPrefix(trimmed, "use_https:") && section == "devices" && currentDevice != nil:
			currentDevice.UseHTTPS = valueAfterColon(trimmed) == "true"

		case strings.HasPrefix(trimmed, "insecure:") && section == "devices" && currentDevice != nil:
			currentDevice.Insecure = valueAfterColon(trimmed) == "true"

		case strings.HasPrefix(trimmed, "poll_interval:"):
			d, err := time.ParseDuration(valueAfterColon(trimmed))
			if err != nil {
				return Config{}, fmt.Errorf("config: invalid poll_interval: %w", err)
			}
			cfg.PollInterval = d

		case strings.HasPrefix(trimmed, "history_size:"):
			n, err := strconv.Atoi(valueAfterColon(trimmed))
			if err != nil {
				return Config{}, fmt.Errorf("config: invalid history_size: %w", err)
			}
			cfg.HistorySize = n

		case strings.HasPrefix(trimmed, "listen_addr:"):
			cfg.ListenAddr = valueAfterColon(trimmed)

		case strings.HasPrefix(trimmed, "web_addr:"):
			cfg.WebAddr = valueAfterColon(trimmed)

		case strings.HasPrefix(trimmed, "sqlite_path:"):
			cfg.SQLitePath = valueAfterColon(trimmed)

		case strings.HasPrefix(trimmed, "notify_threshold:"):
			n, err := strconv.Atoi(valueAfterColon(trimmed))
			if err != nil {
				return Config{}, fmt.Errorf("config: invalid notify_threshold: %w", err)
			}
			cfg.NotifyThreshold = n

		case section == "notifiers" && currentNotifier != nil:
			applyNotifierField(currentNotifier, trimmed)
		}
	}

	flushDevice()
	flushNotifier()

	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("config: reading %s: %w", path, err)
	}

	if len(cfg.Devices) == 0 {
		return Config{}, fmt.Errorf("config: no devices defined in %s", path)
	}

	return cfg, nil
}

// applyDeviceDefaults fills in sensible defaults for fields the
// config file didn't set explicitly — currently just the SNMP
// community string, which nearly every device ships with "public"
// as its read-only default.
func applyDeviceDefaults(d *Device) {
	if d.Type == TypeSNMPFirewall && d.Community == "" {
		d.Community = "public"
	}
}

// applyNotifierField sets one attribute on a Notifier entry from a
// trimmed "key: value" config line. Unrecognized keys are ignored
// rather than erroring, so a typo in one notifier field doesn't take
// down the whole config file.
func applyNotifierField(n *Notifier, trimmed string) {
	switch {
	case strings.HasPrefix(trimmed, "bot_token:"):
		n.BotToken = valueAfterColon(trimmed)
	case strings.HasPrefix(trimmed, "chat_id:"):
		n.ChatID = valueAfterColon(trimmed)
	case strings.HasPrefix(trimmed, "smtp_server:"):
		n.SMTPServer = valueAfterColon(trimmed)
	case strings.HasPrefix(trimmed, "smtp_port:"):
		if port, err := strconv.Atoi(valueAfterColon(trimmed)); err == nil {
			n.SMTPPort = port
		}
	case strings.HasPrefix(trimmed, "username:"):
		n.Username = valueAfterColon(trimmed)
	case strings.HasPrefix(trimmed, "password:"):
		n.Password = valueAfterColon(trimmed)
	case strings.HasPrefix(trimmed, "from:"):
		n.From = valueAfterColon(trimmed)
	case strings.HasPrefix(trimmed, "to:"):
		for _, addr := range strings.Split(valueAfterColon(trimmed), ",") {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				n.To = append(n.To, addr)
			}
		}
	case strings.HasPrefix(trimmed, "url:"):
		n.URL = valueAfterColon(trimmed)
	}
}

// valueAfterColon extracts and trims the value portion of a "key: value"
// or "- name: value" line, also stripping surrounding quotes if present.
func valueAfterColon(line string) string {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return ""
	}
	value := strings.TrimSpace(line[idx+1:])
	value = strings.Trim(value, `"'`)
	return value
}

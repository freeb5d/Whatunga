// Package config loads Whatunga's device list from a simple config
// file. The format is intentionally minimal (not full YAML) so the
// whole tool stays dependency-free — see config.example.yaml for the
// exact syntax this parser expects.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Device describes one RouterOS device to poll.
type Device struct {
	Name     string
	Address  string // host:port, e.g. "192.168.88.1:8728"
	Username string
	Password string
}

// Config is the full set of devices plus global polling settings.
type Config struct {
	PollInterval time.Duration
	HistorySize  int
	ListenAddr   string // REST API (JSON) listen address
	WebAddr      string // browser admin panel listen address
	SQLitePath   string // path to the SQLite database file (users, settings)
	Devices      []Device
}

// Load reads and parses a config file. The expected format is a
// restricted subset of YAML:
//
//	poll_interval: 30s
//	history_size: 120
//	listen_addr: ":8080"
//	web_addr: ":8081"
//	sqlite_path: whatunga.db
//	devices:
//	  - name: office-router
//	    address: 192.168.88.1:8728
//	    username: admin
//	    password: secret
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: opening %s: %w", path, err)
	}
	defer file.Close()

	cfg := Config{
		PollInterval: 30 * time.Second,
		HistorySize:  120,
		ListenAddr:   ":8080",
		WebAddr:      ":8081",
		SQLitePath:   "whatunga.db",
	}

	var current *Device

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "- name:"):
			if current != nil {
				cfg.Devices = append(cfg.Devices, *current)
			}
			current = &Device{Name: valueAfterColon(trimmed)}

		case strings.HasPrefix(trimmed, "address:") && current != nil:
			current.Address = valueAfterColon(trimmed)

		case strings.HasPrefix(trimmed, "username:") && current != nil:
			current.Username = valueAfterColon(trimmed)

		case strings.HasPrefix(trimmed, "password:") && current != nil:
			current.Password = valueAfterColon(trimmed)

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
		}
	}

	if current != nil {
		cfg.Devices = append(cfg.Devices, *current)
	}

	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("config: reading %s: %w", path, err)
	}

	if len(cfg.Devices) == 0 {
		return Config{}, fmt.Errorf("config: no devices defined in %s", path)
	}

	return cfg, nil
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

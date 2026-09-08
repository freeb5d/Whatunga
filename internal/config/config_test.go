package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ParsesDevicesAndSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
poll_interval: 15s
history_size: 60
listen_addr: ":9090"

devices:
  - name: office-router
    address: 192.168.88.1:8728
    username: admin
    password: secret1
  - name: branch-router
    address: 10.0.0.1:8728
    username: monitor
    password: secret2
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.PollInterval != 15*time.Second {
		t.Errorf("PollInterval = %v, want 15s", cfg.PollInterval)
	}
	if cfg.HistorySize != 60 {
		t.Errorf("HistorySize = %d, want 60", cfg.HistorySize)
	}
	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
	}
	if len(cfg.Devices) != 2 {
		t.Fatalf("got %d devices, want 2", len(cfg.Devices))
	}
	if cfg.Devices[0].Name != "office-router" || cfg.Devices[0].Address != "192.168.88.1:8728" {
		t.Errorf("unexpected first device: %+v", cfg.Devices[0])
	}
	if cfg.Devices[1].Username != "monitor" {
		t.Errorf("unexpected second device username: %q", cfg.Devices[1].Username)
	}
}

func TestLoad_MissingFileReturnsError(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatalf("expected an error for a missing config file")
	}
}

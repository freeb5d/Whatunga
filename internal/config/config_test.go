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

func TestLoad_ParsesNotifiersSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
notify_threshold: 5

devices:
  - name: office-router
    address: 192.168.88.1:8728
    username: admin
    password: secret1

notifiers:
  - type: telegram
    bot_token: "123:ABC"
    chat_id: "-100987"
  - type: email
    smtp_server: smtp.example.com
    smtp_port: 587
    username: alerts@example.com
    password: secret
    from: alerts@example.com
    to: ops@example.com, oncall@example.com
  - type: webhook
    url: https://example.com/hooks/whatunga
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.NotifyThreshold != 5 {
		t.Errorf("NotifyThreshold = %d, want 5", cfg.NotifyThreshold)
	}
	if len(cfg.Devices) != 1 {
		t.Fatalf("got %d devices, want 1 (notifiers section shouldn't leak into devices)", len(cfg.Devices))
	}
	if len(cfg.Notifiers) != 3 {
		t.Fatalf("got %d notifiers, want 3: %+v", len(cfg.Notifiers), cfg.Notifiers)
	}

	telegram := cfg.Notifiers[0]
	if telegram.Type != NotifierTelegram || telegram.BotToken != "123:ABC" || telegram.ChatID != "-100987" {
		t.Errorf("unexpected telegram notifier: %+v", telegram)
	}

	email := cfg.Notifiers[1]
	if email.Type != NotifierEmail || email.SMTPPort != 587 || len(email.To) != 2 {
		t.Errorf("unexpected email notifier: %+v", email)
	}
	if email.To[0] != "ops@example.com" || email.To[1] != "oncall@example.com" {
		t.Errorf("unexpected email.To: %+v", email.To)
	}

	webhook := cfg.Notifiers[2]
	if webhook.Type != NotifierWebhook || webhook.URL != "https://example.com/hooks/whatunga" {
		t.Errorf("unexpected webhook notifier: %+v", webhook)
	}
}

func TestLoad_DefaultsNotifyThresholdWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
devices:
  - name: office-router
    address: 192.168.88.1:8728
    username: admin
    password: secret1
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.NotifyThreshold != 3 {
		t.Errorf("NotifyThreshold = %d, want default of 3", cfg.NotifyThreshold)
	}
}

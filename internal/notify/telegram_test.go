package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freeb5d/whatunga/internal/monitor"
)

func TestTelegramNotifier_SendsExpectedFields(t *testing.T) {
	var gotPath string
	var gotChatID, gotText string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		r.ParseForm()
		gotChatID = r.FormValue("chat_id")
		gotText = r.FormValue("text")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	original := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = original }()

	notifier := NewTelegramNotifier("FAKE_TOKEN", "12345")
	err := notifier.Notify(Event{
		Device:  "router1",
		Kind:    monitor.KindRouterOS,
		Status:  StatusDown,
		Message: "connection timeout",
		Time:    time.Now(),
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if gotPath != "/botFAKE_TOKEN/sendMessage" {
		t.Errorf("path = %q, want %q", gotPath, "/botFAKE_TOKEN/sendMessage")
	}
	if gotChatID != "12345" {
		t.Errorf("chat_id = %q, want %q", gotChatID, "12345")
	}
	if !strings.Contains(gotText, "router1") || !strings.Contains(gotText, "DOWN") {
		t.Errorf("text = %q, expected it to mention the device and DOWN status", gotText)
	}
}

func TestTelegramNotifier_NonOKStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	original := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = original }()

	notifier := NewTelegramNotifier("FAKE_TOKEN", "12345")
	err := notifier.Notify(Event{Device: "router1", Status: StatusDown, Time: time.Now()})
	if err == nil {
		t.Fatalf("expected an error for a non-200 response")
	}
}

func TestWebhookNotifier_PostsExpectedJSON(t *testing.T) {
	var received webhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.URL)
	err := notifier.Notify(Event{
		Device:  "firewall1",
		Kind:    monitor.KindSNMPFirewall,
		Status:  StatusDown,
		Message: "no response",
		Time:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if received.Device != "firewall1" {
		t.Errorf("Device = %q, want %q", received.Device, "firewall1")
	}
	if received.Status != "down" {
		t.Errorf("Status = %q, want %q", received.Status, "down")
	}
	if received.Kind != "snmp-firewall" {
		t.Errorf("Kind = %q, want %q", received.Kind, "snmp-firewall")
	}
}

func TestWebhookNotifier_ServerErrorIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.URL)
	err := notifier.Notify(Event{Device: "router1", Status: StatusDown, Time: time.Now()})
	if err == nil {
		t.Fatalf("expected an error for a 500 response")
	}
}

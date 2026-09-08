// This file implements a generic webhook Notifier — a plain HTTP
// POST of the Event as JSON, for wiring Whatunga into anything that
// accepts webhooks (a custom internal dashboard, a chat platform
// other than Telegram, an incident-management tool, etc.).
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookNotifier POSTs a JSON-encoded Event to a configured URL.
type WebhookNotifier struct {
	URL    string
	client *http.Client
}

// NewWebhookNotifier returns a WebhookNotifier ready to use.
func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{
		URL:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// webhookPayload is the JSON shape POSTed to the webhook URL — kept
// as its own type (rather than marshalling Event directly) so the
// wire format is decoupled from Event's internal field names.
type webhookPayload struct {
	Device  string `json:"device"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Time    string `json:"time"`
}

// Notify POSTs the Event as JSON to the configured URL.
func (w *WebhookNotifier) Notify(event Event) error {
	payload := webhookPayload{
		Device:  event.Device,
		Kind:    string(event.Kind),
		Status:  string(event.Status),
		Message: event.Message,
		Time:    event.Time.Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify: webhook: encoding payload: %w", err)
	}

	resp, err := w.client.Post(w.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify: webhook: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// This file implements a Notifier that posts to a Telegram chat via
// the Bot API — just an HTTPS POST of form-encoded fields, so
// net/http is all that's needed; no Telegram SDK required.
package notify

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// telegramAPIBase is the Telegram Bot API base URL. It's a variable
// (not a constant) purely so tests can point it at an httptest
// server instead of the real Telegram API.
var telegramAPIBase = "https://api.telegram.org"

// TelegramNotifier sends alerts to a Telegram chat through a bot.
// Create a bot with @BotFather to get BotToken, and message the bot
// (or add it to a group) to find ChatID — the simplest way is to
// send it any message, then visit
// https://api.telegram.org/bot<token>/getUpdates and read the chat id
// out of the response.
type TelegramNotifier struct {
	BotToken string
	ChatID   string
	client   *http.Client
}

// NewTelegramNotifier returns a TelegramNotifier ready to use.
func NewTelegramNotifier(botToken, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		BotToken: botToken,
		ChatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify posts a formatted alert message for the given Event.
func (t *TelegramNotifier) Notify(event Event) error {
	text := formatMessage(event)

	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, t.BotToken)
	form := url.Values{
		"chat_id":    {t.ChatID},
		"text":       {text},
		"parse_mode": {"HTML"},
	}

	resp, err := t.client.PostForm(apiURL, form)
	if err != nil {
		return fmt.Errorf("notify: telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notify: telegram: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// formatMessage renders an Event as a short HTML-formatted message,
// shared by every Notifier that wants human-readable text (Telegram
// and email both use this).
func formatMessage(event Event) string {
	switch event.Status {
	case StatusDown:
		return fmt.Sprintf(
			"🔴 <b>%s</b> (%s) is DOWN\n%s\nSince: %s",
			event.Device, event.Kind, event.Message, event.Time.Format("2006-01-02 15:04:05 MST"),
		)
	case StatusUp:
		return fmt.Sprintf(
			"🟢 <b>%s</b> (%s) has RECOVERED\nAt: %s",
			event.Device, event.Kind, event.Time.Format("2006-01-02 15:04:05 MST"),
		)
	default:
		return fmt.Sprintf("%s (%s): status changed to %s", event.Device, event.Kind, event.Status)
	}
}

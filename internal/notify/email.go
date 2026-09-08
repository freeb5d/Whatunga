// This file implements a Notifier that sends alerts by email over
// SMTP, using only the standard library's net/smtp — no external
// mail library needed for a simple plain-text alert.
package notify

import (
	"fmt"
	"net/smtp"
	"strings"
)

// EmailNotifier sends alerts via a standard SMTP server. Username
// and Password may be left empty for a relay that doesn't require
// authentication (e.g. an internal mail relay on a trusted network).
type EmailNotifier struct {
	SMTPServer string
	SMTPPort   int
	Username   string
	Password   string
	From       string
	To         []string
}

// NewEmailNotifier returns an EmailNotifier ready to use.
func NewEmailNotifier(server string, port int, username, password, from string, to []string) *EmailNotifier {
	return &EmailNotifier{
		SMTPServer: server,
		SMTPPort:   port,
		Username:   username,
		Password:   password,
		From:       from,
		To:         to,
	}
}

// Notify sends a plain-text email describing the Event.
func (e *EmailNotifier) Notify(event Event) error {
	subject := subjectFor(event)
	body := strings.ReplaceAll(formatMessage(event), "<b>", "")
	body = strings.ReplaceAll(body, "</b>", "")

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n",
		e.From, strings.Join(e.To, ", "), subject, body,
	)

	addr := fmt.Sprintf("%s:%d", e.SMTPServer, e.SMTPPort)

	var auth smtp.Auth
	if e.Username != "" {
		auth = smtp.PlainAuth("", e.Username, e.Password, e.SMTPServer)
	}

	if err := smtp.SendMail(addr, auth, e.From, e.To, []byte(msg)); err != nil {
		return fmt.Errorf("notify: email: %w", err)
	}
	return nil
}

func subjectFor(event Event) string {
	if event.Status == StatusDown {
		return fmt.Sprintf("[Whatunga] %s is DOWN", event.Device)
	}
	return fmt.Sprintf("[Whatunga] %s has recovered", event.Device)
}

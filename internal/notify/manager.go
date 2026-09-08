// Package notify turns raw poll successes/failures into meaningful
// up/down alerts. The key idea (borrowed from how status-page tools
// like sourcegraph/checkup think about this) is that a single failed
// poll is not news — networks blip. What matters is a *streak* of
// consecutive failures crossing a configured threshold, and the
// transition back to healthy afterward. Manager tracks that streak
// per device and fires exactly one notification per transition
// (down, then later recovered) rather than spamming on every poll.
package notify

import (
	"log"
	"sync"
	"time"

	"github.com/freeb5d/whatunga/internal/monitor"
)

// Status is the up/down state Manager tracks per device.
type Status string

const (
	StatusUnknown Status = ""
	StatusUp      Status = "up"
	StatusDown    Status = "down"
)

// Event is what gets handed to every Notifier on a status transition.
type Event struct {
	Device  string
	Kind    monitor.Kind
	Status  Status // the new status: StatusDown or StatusUp (meaning "recovered")
	Message string // the poll error for Down events; empty for Up/recovered
	Time    time.Time
}

// Notifier delivers an Event somewhere — Telegram, email, a generic
// webhook, or anything else that implements this one method.
type Notifier interface {
	Notify(Event) error
}

// deviceState is Manager's per-device bookkeeping.
type deviceState struct {
	status              Status
	consecutiveFailures int
	lastChange          time.Time
	lastError           string
}

// Manager tracks per-device up/down state across polls and fires
// notifications on transitions, once a failure streak reaches
// threshold consecutive failures (to avoid alerting on a single
// transient blip) and again when the device recovers.
type Manager struct {
	mu        sync.Mutex
	threshold int
	notifiers []Notifier
	states    map[string]*deviceState
}

// NewManager returns a Manager that waits for `threshold` consecutive
// failed polls before firing a "down" notification. A threshold of 1
// notifies on the very first failure; most deployments should use
// something like 3 to ride out brief network blips.
func NewManager(threshold int, notifiers []Notifier) *Manager {
	if threshold < 1 {
		threshold = 1
	}
	return &Manager{
		threshold: threshold,
		notifiers: notifiers,
		states:    make(map[string]*deviceState),
	}
}

// RecordSuccess reports that device polled successfully. If the
// device was previously in a Down state (having crossed the failure
// threshold), this fires a "recovered" notification.
func (m *Manager) RecordSuccess(device string, kind monitor.Kind) {
	m.mu.Lock()
	state := m.stateFor(device)
	wasDown := state.status == StatusDown
	state.consecutiveFailures = 0
	state.status = StatusUp
	state.lastChange = time.Now().UTC()
	m.mu.Unlock()

	if wasDown {
		m.fire(Event{Device: device, Kind: kind, Status: StatusUp, Time: time.Now().UTC()})
	}
}

// RecordFailure reports that device failed to poll. Once
// consecutiveFailures reaches the configured threshold, this fires a
// "down" notification exactly once (further failures update the
// streak but don't re-notify until a recovery resets it).
func (m *Manager) RecordFailure(device string, kind monitor.Kind, pollErr error) {
	m.mu.Lock()
	state := m.stateFor(device)
	state.consecutiveFailures++
	state.lastError = pollErr.Error()
	shouldNotify := state.consecutiveFailures == m.threshold && state.status != StatusDown
	if shouldNotify {
		state.status = StatusDown
		state.lastChange = time.Now().UTC()
	}
	message := state.lastError
	m.mu.Unlock()

	if shouldNotify {
		m.fire(Event{Device: device, Kind: kind, Status: StatusDown, Message: message, Time: time.Now().UTC()})
	}
}

// Status returns the current known status of a device (StatusUnknown
// if it's never been polled yet), and when that status last changed.
// This is what the public status page reads.
func (m *Manager) Status(device string) (Status, time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.states[device]
	if !ok {
		return StatusUnknown, time.Time{}
	}
	return state.status, state.lastChange
}

func (m *Manager) stateFor(device string) *deviceState {
	state, ok := m.states[device]
	if !ok {
		state = &deviceState{}
		m.states[device] = state
	}
	return state
}

// fire sends an Event to every configured Notifier. A single
// Notifier's failure (e.g. Telegram's API being briefly unreachable)
// is logged but must not stop the others from getting the alert.
func (m *Manager) fire(event Event) {
	for _, n := range m.notifiers {
		if err := n.Notify(event); err != nil {
			log.Printf("whatunga: notify: %T failed to deliver event for %s: %v", n, event.Device, err)
		}
	}
}

package notify

import (
	"errors"
	"testing"

	"github.com/freeb5d/whatunga/internal/monitor"
)

// fakeNotifier records every Event it receives, for assertions.
type fakeNotifier struct {
	events []Event
}

func (f *fakeNotifier) Notify(e Event) error {
	f.events = append(f.events, e)
	return nil
}

func TestManager_NoNotificationBelowThreshold(t *testing.T) {
	fake := &fakeNotifier{}
	m := NewManager(3, []Notifier{fake})

	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))
	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))

	if len(fake.events) != 0 {
		t.Fatalf("got %d events before reaching threshold, want 0: %v", len(fake.events), fake.events)
	}
}

func TestManager_FiresDownExactlyOnceAtThreshold(t *testing.T) {
	fake := &fakeNotifier{}
	m := NewManager(3, []Notifier{fake})

	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))
	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))
	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout")) // 3rd failure crosses threshold
	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout")) // further failures shouldn't re-notify
	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))

	if len(fake.events) != 1 {
		t.Fatalf("got %d events, want exactly 1 (no spam past threshold): %v", len(fake.events), fake.events)
	}
	if fake.events[0].Status != StatusDown {
		t.Errorf("event status = %q, want %q", fake.events[0].Status, StatusDown)
	}

	status, _ := m.Status("router1")
	if status != StatusDown {
		t.Errorf("Status() = %q, want %q", status, StatusDown)
	}
}

func TestManager_FiresRecoveredAfterDown(t *testing.T) {
	fake := &fakeNotifier{}
	m := NewManager(2, []Notifier{fake})

	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))
	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout")) // crosses threshold -> down event
	m.RecordSuccess("router1", monitor.KindRouterOS)                       // recovers -> up event

	if len(fake.events) != 2 {
		t.Fatalf("got %d events, want 2 (down, then recovered): %v", len(fake.events), fake.events)
	}
	if fake.events[1].Status != StatusUp {
		t.Errorf("second event status = %q, want %q", fake.events[1].Status, StatusUp)
	}

	status, _ := m.Status("router1")
	if status != StatusUp {
		t.Errorf("Status() after recovery = %q, want %q", status, StatusUp)
	}
}

func TestManager_SuccessesBeforeAnyFailureNeverNotify(t *testing.T) {
	fake := &fakeNotifier{}
	m := NewManager(3, []Notifier{fake})

	m.RecordSuccess("router1", monitor.KindRouterOS)
	m.RecordSuccess("router1", monitor.KindRouterOS)

	if len(fake.events) != 0 {
		t.Fatalf("got %d events for a device that never failed, want 0: %v", len(fake.events), fake.events)
	}
}

func TestManager_FailureStreakResetsAfterRecovery(t *testing.T) {
	fake := &fakeNotifier{}
	m := NewManager(2, []Notifier{fake})

	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))
	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout")) // down (event 1)
	m.RecordSuccess("router1", monitor.KindRouterOS)                       // recovered (event 2)

	// A single failure after recovery should NOT immediately re-notify,
	// since the streak should have reset to zero.
	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))

	if len(fake.events) != 2 {
		t.Fatalf("got %d events after a single post-recovery failure, want 2 (streak should have reset): %v", len(fake.events), fake.events)
	}
}

func TestManager_DevicesAreIndependent(t *testing.T) {
	fake := &fakeNotifier{}
	m := NewManager(1, []Notifier{fake})

	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))

	status, _ := m.Status("router2")
	if status != StatusUnknown {
		t.Errorf("unrelated device status = %q, want %q (unknown)", status, StatusUnknown)
	}
}

func TestManager_NotifierErrorDoesNotStopOthers(t *testing.T) {
	failing := &erroringNotifier{}
	fake := &fakeNotifier{}
	m := NewManager(1, []Notifier{failing, fake})

	m.RecordFailure("router1", monitor.KindRouterOS, errors.New("timeout"))

	if len(fake.events) != 1 {
		t.Fatalf("second notifier got %d events, want 1 despite the first notifier failing", len(fake.events))
	}
}

type erroringNotifier struct{}

func (erroringNotifier) Notify(Event) error { return errors.New("delivery failed") }

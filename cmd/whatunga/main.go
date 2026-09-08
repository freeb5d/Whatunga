// Whatunga is a CLI + REST API + browser admin panel for polling
// MikroTik RouterOS, HPE iLO, Windows Server, and SNMP-capable
// firewalls.
//
// Usage:
//
//	whatunga status  -config config.yaml   # one-off poll of all devices, printed to stdout
//	whatunga watch    -config config.yaml   # continuous polling, printed to stdout on each tick
//	whatunga serve    -config config.yaml   # continuous polling + JSON REST API + browser admin panel
//
// Each device entry in the config file has a "type" — routeros, ilo,
// windows, or snmp-firewall — selecting which of the four Poller
// implementations in internal/monitor handles it. See
// internal/config/config.go's doc comment for the full config
// format, and config.example.yaml for a worked example of every type.
//
// The "serve" command starts two HTTP servers: a JSON REST API
// (cfg.ListenAddr) and a browser-based admin panel (cfg.WebAddr) with
// sign-in, live device status, a language switcher (English, Te Reo
// Māori, French, Chinese), an Account page for changing the admin
// password, and an unauthenticated public status page at /status.
// The admin panel's user and settings live in a small SQLite
// database (cfg.SQLitePath), seeded on first run with the default
// admin/admin credentials — change these immediately from the
// Account page.
//
// Both "watch" and "serve" also feed every poll result into a
// notify.Manager, which fires alerts (Telegram/email/webhook, per
// the "notifiers" section of the config file) once a device's
// consecutive-failure count crosses cfg.NotifyThreshold, and again
// when it recovers.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/freeb5d/whatunga/internal/api"
	"github.com/freeb5d/whatunga/internal/config"
	"github.com/freeb5d/whatunga/internal/db"
	"github.com/freeb5d/whatunga/internal/monitor"
	"github.com/freeb5d/whatunga/internal/notify"
	"github.com/freeb5d/whatunga/internal/store"
	"github.com/freeb5d/whatunga/internal/webui"
)

const pollTimeout = 5 * time.Second

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	fs := flag.NewFlagSet(subcommand, flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "path to config file")
	fs.Parse(os.Args[2:])

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("whatunga: %v", err)
	}

	switch subcommand {
	case "status":
		runStatus(cfg)
	case "watch":
		runWatch(cfg)
	case "serve":
		runServe(cfg)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: whatunga <status|watch|serve> [-config config.yaml]")
}

// newPollerFor is the single factory dispatching a config.Device to
// the right monitor.Poller implementation by its Type. Adding a new
// device kind means adding one case here (and, of course, writing
// the Poller implementation itself in internal/monitor).
func newPollerFor(d config.Device) (monitor.Poller, error) {
	switch d.Type {
	case config.TypeILO:
		return monitor.NewILOPoller(d.Name, d.Address, d.Username, d.Password, d.Insecure, pollTimeout), nil

	case config.TypeWindows:
		return monitor.NewWindowsPoller(d.Name, d.Address, d.Username, d.Password, d.UseHTTPS, d.Insecure, pollTimeout)

	case config.TypeSNMPFirewall:
		return monitor.NewSNMPFirewallPoller(d.Name, d.Address, d.Community, pollTimeout), nil

	case config.TypeRouterOS, "":
		return monitor.NewRouterOSPoller(d.Name, d.Address, d.Username, d.Password, pollTimeout)

	default:
		return nil, fmt.Errorf("unknown device type %q", d.Type)
	}
}

// deviceKind maps a config.DeviceType to the equivalent monitor.Kind
// — the two are separate types (so internal/config never has to
// import internal/monitor) but share the same underlying string
// values, so this is a direct conversion plus the same "" defaults
// to routeros" rule newPollerFor uses.
func deviceKind(t config.DeviceType) monitor.Kind {
	if t == "" {
		return monitor.KindRouterOS
	}
	return monitor.Kind(t)
}

// buildNotifiers turns the config file's "notifiers" section into
// concrete notify.Notifier implementations.
func buildNotifiers(cfg config.Config) []notify.Notifier {
	var notifiers []notify.Notifier

	for _, n := range cfg.Notifiers {
		switch n.Type {
		case config.NotifierTelegram:
			notifiers = append(notifiers, notify.NewTelegramNotifier(n.BotToken, n.ChatID))
		case config.NotifierEmail:
			notifiers = append(notifiers, notify.NewEmailNotifier(n.SMTPServer, n.SMTPPort, n.Username, n.Password, n.From, n.To))
		case config.NotifierWebhook:
			notifiers = append(notifiers, notify.NewWebhookNotifier(n.URL))
		default:
			log.Printf("whatunga: unknown notifier type %q, skipping", n.Type)
		}
	}

	return notifiers
}

// pollerEntry pairs a live Poller with its device kind, so pollAllOnce
// and pollAllInto can report failures/successes to notify.Manager
// without needing to look the kind up elsewhere.
type pollerEntry struct {
	poller monitor.Poller
	kind   monitor.Kind
}

// runStatus polls every configured device exactly once and prints the
// resulting snapshots as pretty-printed JSON.
func runStatus(cfg config.Config) {
	for _, device := range cfg.Devices {
		snap, err := pollOnce(device)
		if err != nil {
			log.Printf("whatunga: %s: %v", device.Name, err)
			continue
		}
		printSnapshot(snap)
	}
}

// runWatch polls every configured device on a repeating interval and
// prints each snapshot as it comes in. It keeps its own Poller per
// device open for the lifetime of the process rather than reconnecting
// on every tick, and reports every success/failure to a notify.Manager
// so configured alerts (Telegram/email/webhook) still fire even when
// just running "watch" rather than the full "serve".
func runWatch(cfg config.Config) {
	pollers := openPollers(cfg.Devices)
	defer closePollers(pollers)

	manager := notify.NewManager(cfg.NotifyThreshold, buildNotifiers(cfg))

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	pollAllOnce(pollers, manager)
	for range ticker.C {
		pollAllOnce(pollers, manager)
	}
}

// runServe polls every configured device on a repeating interval,
// keeps a bounded history of the results in memory, feeds every
// poll result into a notify.Manager for alerting, and serves the
// results two ways: a JSON REST API and a browser-based admin panel
// backed by a small SQLite database (admin user + UI settings).
func runServe(cfg config.Config) {
	conn, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("whatunga: opening database: %v", err)
	}
	defer conn.Close()

	log.Printf(
		"whatunga: admin panel default login is %q / %q — change this from the Account page immediately",
		db.DefaultAdminUsername, db.DefaultAdminPassword,
	)

	history := store.NewHistory(cfg.HistorySize)
	manager := notify.NewManager(cfg.NotifyThreshold, buildNotifiers(cfg))
	pollers := openPollers(cfg.Devices)
	defer closePollers(pollers)

	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()

		pollAllInto(pollers, history, manager)
		for range ticker.C {
			pollAllInto(pollers, history, manager)
		}
	}()

	apiServer := api.NewServer(history)
	go func() {
		log.Printf("whatunga: serving REST API on %s", cfg.ListenAddr)
		if err := http.ListenAndServe(cfg.ListenAddr, apiServer); err != nil {
			log.Fatalf("whatunga: REST API server error: %v", err)
		}
	}()

	webServer := webui.NewServer(conn, history, manager)
	log.Printf("whatunga: serving admin panel on %s (public status page at /status)", cfg.WebAddr)
	if err := http.ListenAndServe(cfg.WebAddr, webServer); err != nil {
		log.Fatalf("whatunga: admin panel server error: %v", err)
	}
}

func openPollers(devices []config.Device) map[string]pollerEntry {
	pollers := make(map[string]pollerEntry, len(devices))
	for _, d := range devices {
		poller, err := newPollerFor(d)
		if err != nil {
			log.Printf("whatunga: could not set up %s (%s): %v", d.Name, d.Type, err)
			continue
		}
		pollers[d.Name] = pollerEntry{poller: poller, kind: deviceKind(d.Type)}
	}
	return pollers
}

func closePollers(pollers map[string]pollerEntry) {
	for _, entry := range pollers {
		entry.poller.Close()
	}
}

func pollOnce(device config.Device) (monitor.Snapshot, error) {
	poller, err := newPollerFor(device)
	if err != nil {
		return monitor.Snapshot{}, err
	}
	defer poller.Close()
	return poller.Snapshot()
}

func pollAllOnce(pollers map[string]pollerEntry, manager *notify.Manager) {
	for name, entry := range pollers {
		snap, err := entry.poller.Snapshot()
		if err != nil {
			log.Printf("whatunga: %s: poll failed: %v", name, err)
			manager.RecordFailure(name, entry.kind, err)
			continue
		}
		manager.RecordSuccess(name, entry.kind)
		printSnapshot(snap)
	}
}

func pollAllInto(pollers map[string]pollerEntry, history *store.History, manager *notify.Manager) {
	for name, entry := range pollers {
		snap, err := entry.poller.Snapshot()
		if err != nil {
			log.Printf("whatunga: %s: poll failed: %v", name, err)
			manager.RecordFailure(name, entry.kind, err)
			continue
		}
		manager.RecordSuccess(name, entry.kind)
		history.Add(snap)
	}
}

func printSnapshot(snap monitor.Snapshot) {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		log.Printf("whatunga: marshalling snapshot: %v", err)
		return
	}
	fmt.Println(string(data))
}

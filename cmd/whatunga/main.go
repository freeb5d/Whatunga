// Whatunga is a small, dependency-free CLI + REST API for polling
// MikroTik RouterOS devices over their native API protocol.
//
// Usage:
//
//	whatunga status  -config config.yaml   # one-off poll of all devices, printed to stdout
//	whatunga watch   -config config.yaml   # continuous polling, printed to stdout on each tick
//	whatunga serve   -config config.yaml   # continuous polling + JSON REST API + browser admin panel
//
// The "serve" command starts two HTTP servers: a JSON REST API
// (cfg.ListenAddr) and a browser-based admin panel (cfg.WebAddr) with
// sign-in, live device status, a language switcher (English, Te Reo
// Māori, French, Chinese), and an Account page for changing the admin
// password. The admin panel's user and settings live in a small
// SQLite database (cfg.SQLitePath), seeded on first run with the
// default admin/admin credentials — change these immediately from
// the Account page.
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
	"github.com/freeb5d/whatunga/internal/store"
	"github.com/freeb5d/whatunga/internal/webui"
)

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
// on every tick.
func runWatch(cfg config.Config) {
	pollers := openPollers(cfg.Devices)
	defer closePollers(pollers)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	pollAllOnce(pollers)
	for range ticker.C {
		pollAllOnce(pollers)
	}
}

// runServe polls every configured device on a repeating interval,
// keeps a bounded history of the results in memory, and serves that
// history two ways: a JSON REST API and a browser-based admin panel
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
	pollers := openPollers(cfg.Devices)
	defer closePollers(pollers)

	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()

		pollAllInto(pollers, history)
		for range ticker.C {
			pollAllInto(pollers, history)
		}
	}()

	apiServer := api.NewServer(history)
	go func() {
		log.Printf("whatunga: serving REST API on %s", cfg.ListenAddr)
		if err := http.ListenAndServe(cfg.ListenAddr, apiServer); err != nil {
			log.Fatalf("whatunga: REST API server error: %v", err)
		}
	}()

	webServer := webui.NewServer(conn, history)
	log.Printf("whatunga: serving admin panel on %s", cfg.WebAddr)
	if err := http.ListenAndServe(cfg.WebAddr, webServer); err != nil {
		log.Fatalf("whatunga: admin panel server error: %v", err)
	}
}

func openPollers(devices []config.Device) map[string]*monitor.Poller {
	pollers := make(map[string]*monitor.Poller, len(devices))
	for _, d := range devices {
		poller, err := monitor.NewPoller(d.Name, d.Address, d.Username, d.Password, 5*time.Second)
		if err != nil {
			log.Printf("whatunga: could not connect to %s (%s): %v", d.Name, d.Address, err)
			continue
		}
		pollers[d.Name] = poller
	}
	return pollers
}

func closePollers(pollers map[string]*monitor.Poller) {
	for _, p := range pollers {
		p.Close()
	}
}

func pollOnce(device config.Device) (monitor.Snapshot, error) {
	poller, err := monitor.NewPoller(device.Name, device.Address, device.Username, device.Password, 5*time.Second)
	if err != nil {
		return monitor.Snapshot{}, err
	}
	defer poller.Close()
	return poller.Snapshot()
}

func pollAllOnce(pollers map[string]*monitor.Poller) {
	for name, poller := range pollers {
		snap, err := poller.Snapshot()
		if err != nil {
			log.Printf("whatunga: %s: poll failed: %v", name, err)
			continue
		}
		printSnapshot(snap)
	}
}

func pollAllInto(pollers map[string]*monitor.Poller, history *store.History) {
	for name, poller := range pollers {
		snap, err := poller.Snapshot()
		if err != nil {
			log.Printf("whatunga: %s: poll failed: %v", name, err)
			continue
		}
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

# Whatunga 🕸️📡

[![release](https://img.shields.io/github/v/release/freeb5d/Whatunga?label=release&color=2b3137)](https://github.com/freeb5d/Whatunga/releases)
[![build](https://img.shields.io/github/actions/workflow/status/freeb5d/Whatunga/release.yml?branch=main&label=build)](https://github.com/freeb5d/Whatunga/actions/workflows/release.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/freeb5d/Whatunga?label=Go&color=00ADD8)](go.mod)
[![downloads](https://img.shields.io/github/downloads/freeb5d/Whatunga/total?label=downloads&color=brightgreen)](https://github.com/freeb5d/Whatunga/releases)
[![license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![docs](https://img.shields.io/badge/docs-README-informational)](#whatunga-)

*"Whatunga"* is the Māori word for **network** — a fitting name for a tool that watches over one. Whatunga is a CLI + REST API + **browser admin panel** written in Go for monitoring **MikroTik RouterOS** devices — the kind of infrastructure work that comes up constantly in real network/sysadmin roles (RouterOS, VMware ESXi, MikroTik-based branch networks).

## Why this project

Most "monitoring tool" tutorials wrap an existing client library around a REST call. This project does the more interesting (and more revealing) thing: it implements MikroTik's **binary API protocol** from scratch, straight from the wire-format spec — variable-length word encoding, sentence framing, login handshake — with no library standing in for that part. Everything else (the web panel, the REST API, the config parser) is also built on the Go standard library rather than a framework, with only two well-known third-party packages pulled in for the two jobs the standard library genuinely doesn't cover: SQLite storage and password hashing.

- **`internal/routeros`** — hand-written RouterOS API client (TCP, binary protocol, no library)
- **`internal/monitor`** — turns raw API replies (which are just string maps on the wire) into typed Go structs
- **`internal/store`** — a small thread-safe ring buffer keeping recent history per device, in memory
- **`internal/api`** — a JSON REST API over `net/http`, no router framework needed for four small routes
- **`internal/webui`** — the browser admin panel: sign-in, live dashboard, language switcher, change-password page — templates and CSS embedded into the binary via `//go:embed`
- **`internal/db`** — SQLite persistence (admin user + settings) via `modernc.org/sqlite`, a pure-Go driver, so the binary stays cgo-free
- **`internal/i18n`** — flat key/value translations for **English, Te Reo Māori, French, and Chinese (Simplified)**
- **`internal/config`** — a minimal hand-rolled YAML-subset parser (no dependency)
- **`cmd/whatunga`** — the CLI: `status`, `watch`, `serve`

## Admin panel

`whatunga serve` starts two servers: the JSON REST API (`listen_addr`) and a browser admin panel (`web_addr`). The panel is backed by a small SQLite database (`sqlite_path`) holding exactly one thing beyond the schema itself — the admin user's bcrypt password hash — plus a one-row settings table for the UI's default language.

- **Default login**: `admin` / `admin`, seeded automatically the first time the database is created. **Change this immediately** from the Account page after your first sign-in — the server also logs a reminder on every `serve` startup until you do.
- **Language switcher**: a dropdown in the top bar and on the login page lets you switch between English, Te Reo Māori, Français, and 中文 at any time; the choice is remembered both as a cookie and as the server-wide default (so a fresh browser session still opens in whichever language was last chosen).
- **Sessions**: simple, server-side, in-memory session tokens in an HttpOnly cookie — appropriate for a single-admin tool like this, not meant to be a general-purpose auth system for many users.

## Why RouterOS specifically

MikroTik RouterOS runs a huge amount of small/branch business networking infrastructure. Its API (port 8728, or 8729 for TLS) is a simple length-prefixed binary sentence protocol — documented by MikroTik but rarely implemented from scratch in portfolio projects, which tend to just shell out to `winbox` or use an existing Go/Python client library.

## Quick install (Linux servers)

For a production/server install, skip building from source entirely — a single command downloads the right binary for your machine, sets it up as a systemd service, and gets you to a running admin panel in under a minute:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/freeb5d/Whatunga/main/install.sh)
```

Running it with no arguments opens an interactive menu:

```
Whatunga installer
------------------
1) Install
2) Update to latest release
3) Uninstall
4) Service status
0) Exit
```

What each option does:

| Option | What it does |
|---|---|
| **1) Install** | Detects your OS/architecture, downloads the matching binary from the [latest GitHub release](https://github.com/freeb5d/Whatunga/releases), installs it to `/usr/local/bin/whatunga`, creates a starter config at `/etc/whatunga/config.yaml` (only if one doesn't already exist), and registers + enables a `whatunga.service` systemd unit so it survives reboots. |
| **2) Update to latest release** | Stops the running service, re-downloads and re-installs the newest release binary over the old one, then restarts the service. Your config and database in `/etc/whatunga` are left untouched. |
| **3) Uninstall** | Stops and disables the service, removes the systemd unit file and the binary. Your config/database are **not** deleted automatically — it prints the exact `rm -rf` command if you want a full wipe. |
| **4) Service status** | Runs `systemctl status whatunga` so you can check it's healthy without remembering the command yourself. |

You can also skip the menu entirely and call a step directly — useful for scripting or a fresh server bootstrap:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/freeb5d/Whatunga/main/install.sh) install
```

After installing, the only manual step left is pointing it at your real device(s):

```bash
sudo nano /etc/whatunga/config.yaml   # add your RouterOS device address, username, password
sudo systemctl start whatunga
sudo systemctl status whatunga        # confirm it's "active (running)"
```

Then open:
- **Admin panel** → `http://<server-ip>:8081` (default login `admin` / `admin` — change it immediately from the Account page)
- **JSON API** → `http://<server-ip>:8080/api/v1/devices`

Note: this installer targets **Linux servers with systemd** (the same audience as most one-line `curl | bash` installers). It won't run on Windows — use the "building from source" instructions below on Windows/macOS, or run it inside WSL.

## Getting started (building from source)

This repo's `go.mod` declares two third-party dependencies (`modernc.org/sqlite` and `golang.org/x/crypto`) but, since it was put together without internet access in this environment, **`go.sum` is not included**. Run `go mod tidy` once — with internet access — before your first build; it will fetch both packages (and modernc.org/sqlite's own small dependency tree) and generate `go.sum` for you.

```bash
go mod tidy               # fetches dependencies, generates go.sum (needs internet)
go build -o bin/whatunga ./cmd/whatunga
# or: make build

cp config.example.yaml config.yaml
# edit config.yaml with your real RouterOS device(s) — address, username, password
```

### One-off status check

```bash
./bin/whatunga status -config config.yaml
```

Prints a JSON snapshot (system resource + all interfaces) for every configured device.

### Continuous polling to stdout

```bash
./bin/whatunga watch -config config.yaml
```

### REST API + admin panel

```bash
./bin/whatunga serve -config config.yaml
```

This starts both servers. The JSON API:

```bash
curl localhost:8080/api/v1/devices
curl localhost:8080/api/v1/devices/office-router
curl localhost:8080/api/v1/devices/office-router/history
curl localhost:8080/healthz
```

And the admin panel — open `http://localhost:8081` in a browser, sign in with `admin` / `admin`, and change the password from the Account page.

## Running the tests

```bash
go test ./... -v
# or: make test
```

Notably, `internal/routeros/client_test.go` spins up an **in-process fake RouterOS server** (a real `net.Listener` speaking just enough of the wire protocol) to test the full `Dial` → `login` → `Run` flow without needing a real MikroTik device on hand. `internal/routeros/protocol_test.go` round-trips every word-length encoding boundary (1-byte through 5-byte) to make sure the variable-length scheme is implemented correctly at each size class. `internal/db/db_test.go` runs the same checks against an in-memory SQLite database (`:memory:`), covering the default-admin seed, correct/incorrect login, and that a password change actually invalidates the old password.

## Config format

See `config.example.yaml`. It's a deliberately small subset of YAML — enough for a flat list of devices and a few settings, parsed with `bufio.Scanner` rather than pulling in a YAML library. This keeps the whole tool a single static binary with no runtime dependencies.

## Security notes (honest scope limits)

This is a portfolio-scale admin panel, not a hardened multi-tenant auth system. Before using it beyond a local/trusted network:

- Put it behind TLS (a reverse proxy like Caddy or nginx is the simplest route) — the login form currently posts credentials in the clear over plain HTTP.
- There's no CSRF token on the login/account forms yet — low risk for a single-admin tool behind auth, but worth adding (`gorilla/csrf` or a hand-rolled token) if you expose this beyond localhost.
- There's no rate-limiting on `/login` — consider adding it if the panel is reachable from the internet.

## Roadmap ideas (good next PRs for this portfolio piece)

- [ ] API-SSL (port 8729, TLS) support in the `routeros` client
- [ ] Prometheus `/metrics` endpoint alongside the JSON API
- [ ] Alerting (webhook or email) when CPU load or interface state crosses a threshold
- [ ] VMware vSphere poller alongside the RouterOS one, behind a common `Poller` interface
- [ ] CSRF protection and login rate-limiting on the admin panel (see Security notes above)
- [ ] Multi-user support (currently a single seeded admin account)

## License

MIT — free to use as a base for your own projects.

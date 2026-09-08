// Package webui is Whatunga's browser-based admin panel: sign in,
// view live device status, switch UI language, and change the admin
// password.
package webui

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/freeb5d/whatunga/internal/db"
	"github.com/freeb5d/whatunga/internal/i18n"
	"github.com/freeb5d/whatunga/internal/monitor"
	"github.com/freeb5d/whatunga/internal/notify"
	"github.com/freeb5d/whatunga/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*.css
var staticFS embed.FS

const sessionCookieName = "whatunga_session"

// Server serves the web admin panel.
type Server struct {
	db       *sql.DB
	history  *store.History
	manager  *notify.Manager
	sessions *sessionManager
	mux      *http.ServeMux
}

// NewServer builds a Server backed by the given SQLite connection,
// device history store, and notify.Manager (used for the public
// status page's up/down state), and registers all routes.
func NewServer(conn *sql.DB, history *store.History, manager *notify.Manager) *Server {
	s := &Server{
		db:       conn,
		history:  history,
		manager:  manager,
		sessions: newSessionManager(24 * time.Hour),
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

// ServeHTTP lets Server itself be used as an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	s.mux.HandleFunc("/login", s.handleLogin)
	s.mux.HandleFunc("/logout", s.handleLogout)
	s.mux.HandleFunc("/lang", s.handleSetLanguage)

	// /status is intentionally unauthenticated — a read-only summary
	// meant to be shared with people who shouldn't get admin access
	// (a wider team, or an external status page link), same idea as
	// sourcegraph/checkup's public status pages. It shows only
	// up/down state, not full poll detail.
	s.mux.HandleFunc("/status", s.handlePublicStatus)

	s.mux.HandleFunc("/", s.requireAuth(s.handleDashboard))
	s.mux.HandleFunc("/account", s.requireAuth(s.handleAccount))
}

// viewData is the common template context every page renders with.
type viewData struct {
	Lang          string
	Languages     []i18n.Language
	Authenticated bool
	Username      string
	Flash         string
	FlashKind     string // "success" or "error"

	// Dashboard-only fields.
	Devices []monitor.Snapshot

	// Public status page-only fields.
	StatusDevices []publicDeviceStatus
}

// publicDeviceStatus is the minimal, non-sensitive view of a device
// shown on the unauthenticated /status page — just up/down and when
// that last changed, no credentials, addresses, or detailed metrics.
type publicDeviceStatus struct {
	Device     string
	Kind       monitor.Kind
	Status     notify.Status
	LastChange time.Time
}

func (s *Server) currentLanguage(r *http.Request) string {
	if cookie, err := r.Cookie("lang"); err == nil && i18n.IsSupported(cookie.Value) {
		return cookie.Value
	}
	return db.GetSetting(s.db, "language", "en")
}

func (s *Server) currentUser(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	sess, ok := s.sessions.lookup(cookie.Value)
	if !ok {
		return "", false
	}
	return sess.username, true
}

// requireAuth wraps a handler so it redirects to /login when there is
// no valid session cookie.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.currentUser(r); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := s.baseViewData(r)

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if err := db.Authenticate(s.db, username, password); err != nil {
			data.Flash = i18n.T(data.Lang, "invalid_credentials")
			data.FlashKind = "error"
			s.render(w, "login.html", data)
			return
		}

		token, err := s.sessions.create(username)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(24 * time.Hour),
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	s.render(w, "login.html", data)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.sessions.destroy(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleSetLanguage(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if i18n.IsSupported(code) {
		http.SetCookie(w, &http.Cookie{
			Name:    "lang",
			Value:   code,
			Path:    "/",
			Expires: time.Now().Add(365 * 24 * time.Hour),
		})
		// Also persist as the server-wide default so a fresh browser
		// (no cookie yet) still gets the admin's last chosen language.
		_ = db.SetSetting(s.db, "language", code)
	}

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := s.baseViewData(r)

	for _, name := range s.history.Devices() {
		snap, ok := s.history.Latest(name)
		if !ok {
			continue
		}
		data.Devices = append(data.Devices, snap)
	}

	s.render(w, "dashboard.html", data)
}

// handlePublicStatus renders the unauthenticated status page: just
// each known device's up/down state and when that last changed,
// derived from notify.Manager rather than the full Snapshot history
// (which stays admin-only, since it can include internal addresses
// and other detail not meant for a public audience).
func (s *Server) handlePublicStatus(w http.ResponseWriter, r *http.Request) {
	data := s.baseViewData(r)

	for _, name := range s.history.Devices() {
		snap, ok := s.history.Latest(name)
		if !ok {
			continue
		}
		status, lastChange := s.manager.Status(name)
		data.StatusDevices = append(data.StatusDevices, publicDeviceStatus{
			Device:     name,
			Kind:       snap.Kind,
			Status:     status,
			LastChange: lastChange,
		})
	}

	s.render(w, "status.html", data)
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	data := s.baseViewData(r)
	username, _ := s.currentUser(r)

	if r.Method == http.MethodPost {
		current := r.FormValue("current_password")
		newPass := r.FormValue("new_password")
		confirm := r.FormValue("confirm_password")

		switch {
		case len(newPass) < 8:
			data.Flash = i18n.T(data.Lang, "password_too_short")
			data.FlashKind = "error"
		case newPass != confirm:
			data.Flash = i18n.T(data.Lang, "password_mismatch")
			data.FlashKind = "error"
		default:
			if err := db.ChangePassword(s.db, username, current, newPass); err != nil {
				if err == db.ErrIncorrectCurrentPassword {
					data.Flash = i18n.T(data.Lang, "incorrect_current_password")
				} else {
					data.Flash = err.Error()
				}
				data.FlashKind = "error"
			} else {
				data.Flash = i18n.T(data.Lang, "password_updated")
				data.FlashKind = "success"
			}
		}
	}

	s.render(w, "account.html", data)
}

func (s *Server) baseViewData(r *http.Request) viewData {
	lang := s.currentLanguage(r)
	username, authenticated := s.currentUser(r)

	return viewData{
		Lang:          lang,
		Languages:     i18n.SupportedLanguages(),
		Authenticated: authenticated,
		Username:      username,
	}
}

// render parses and executes a page template together with the shared
// layout, injecting a T(key) translation function bound to the
// request's current language.
func (s *Server) render(w http.ResponseWriter, page string, data any) {
	lang := "en"
	if vd, ok := data.(viewData); ok {
		lang = vd.Lang
	}

	funcs := template.FuncMap{
		"T": func(key string) string { return i18n.T(lang, key) },
		"cpuClass": func(load int) string {
			switch {
			case load >= 80:
				return "high"
			case load >= 50:
				return "medium"
			default:
				return "low"
			}
		},
		"formatBytes": formatBytes,
		"kbToBytes":   func(kb int64) int64 { return kb * 1024 },
		"kindLabel": func(kind monitor.Kind) string {
			return i18n.T(lang, "kind_"+strings.ReplaceAll(string(kind), "-", "_"))
		},
		"healthClass": func(health string) string {
			switch strings.ToLower(health) {
			case "ok":
				return "ok"
			case "warning":
				return "warning"
			default:
				return "critical"
			}
		},
		"statusLabel": func(status notify.Status) string {
			switch status {
			case notify.StatusUp:
				return i18n.T(lang, "status_up")
			case notify.StatusDown:
				return i18n.T(lang, "status_down")
			default:
				return i18n.T(lang, "status_unknown")
			}
		},
		"statusClass": func(status notify.Status) string {
			switch status {
			case notify.StatusUp:
				return "up"
			case notify.StatusDown:
				return "down"
			default:
				return "unknown"
			}
		},
	}

	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/layout.html", "templates/"+page)
	if err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, fmt.Sprintf("render error: %v", err), http.StatusInternalServerError)
	}
}

// formatBytes renders a byte count in a human-friendly unit, used by
// the interface traffic columns on the dashboard.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	units := "KMGTPE"
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), units[exp])
}



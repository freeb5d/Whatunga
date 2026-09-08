package webui

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// session holds what the web UI needs to know about a signed-in user.
type session struct {
	username string
	expires  time.Time
}

// sessionManager is a minimal in-memory session store, appropriate for
// a single-admin tool like this one. Tokens are opaque random strings
// held server-side (not JWTs) — there's no distributed state to
// synchronize, so there's nothing a self-contained signed token would
// buy over a plain server-side map.
type sessionManager struct {
	mu       sync.Mutex
	sessions map[string]session
	ttl      time.Duration
}

func newSessionManager(ttl time.Duration) *sessionManager {
	return &sessionManager{
		sessions: make(map[string]session),
		ttl:      ttl,
	}
}

// create starts a new session for username and returns its token.
func (m *sessionManager) create(username string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	m.sessions[token] = session{username: username, expires: time.Now().Add(m.ttl)}
	m.mu.Unlock()

	return token, nil
}

// lookup returns the session for token if it exists and hasn't expired.
func (m *sessionManager) lookup(token string) (session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[token]
	if !ok {
		return session{}, false
	}
	if time.Now().After(sess.expires) {
		delete(m.sessions, token)
		return session{}, false
	}
	return sess, true
}

// destroy ends a session (used on logout).
func (m *sessionManager) destroy(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

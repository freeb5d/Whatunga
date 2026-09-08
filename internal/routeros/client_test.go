package routeros

import (
	"bufio"
	"net"
	"testing"
	"time"
)

// startFakeRouterOS starts a minimal in-process server that speaks just
// enough of the RouterOS API protocol to exercise Client.Dial and
// Client.Run without needing a real MikroTik device. It accepts one
// connection, replies "done" to /login, and then to any command it
// echoes back a single canned row followed by !done.
func startFakeRouterOS(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting fake server: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)

		// 1. Handle /login.
		if _, err := readSentence(r); err != nil {
			return
		}
		writeSentence(conn, []string{"!done"})

		// 2. Handle whatever command comes next: reply with one fake row.
		if _, err := readSentence(r); err != nil {
			return
		}
		writeSentence(conn, []string{"!re", "=name=ether1", "=running=true"})
		writeSentence(conn, []string{"!done"})
	}()

	return listener.Addr().String()
}

func TestClient_DialLoginAndRun(t *testing.T) {
	addr := startFakeRouterOS(t)

	client, err := Dial(addr, "admin", "password", 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	reply, err := client.Run("/interface/print")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if reply.Status != "done" {
		t.Fatalf("status = %q, want %q", reply.Status, "done")
	}
	if len(reply.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(reply.Rows))
	}
	if reply.Rows[0]["name"] != "ether1" {
		t.Errorf("name = %q, want %q", reply.Rows[0]["name"], "ether1")
	}
	if reply.Rows[0]["running"] != "true" {
		t.Errorf("running = %q, want %q", reply.Rows[0]["running"], "true")
	}
}

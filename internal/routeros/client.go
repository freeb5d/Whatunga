package routeros

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

// Client is a connection to a RouterOS device's API service.
// It is not safe for concurrent use by multiple goroutines — callers
// that need concurrent access should serialize their own calls or open
// multiple Clients.
type Client struct {
	conn net.Conn
	r    *bufio.Reader
}

// Dial connects to a RouterOS device at address (host:port, typically
// port 8728 for the plain API) and logs in with the given credentials.
//
// RouterOS versions from 6.43 onward accept a plain-text login sentence
// (the username/password are still protected because this should always
// be run over a trusted management network or the API-SSL port 8729 —
// this client does not implement the legacy MD5-challenge login used by
// pre-6.43 firmware).
func Dial(address, username, password string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, fmt.Errorf("routeros: dial %s: %w", address, err)
	}

	c := &Client{conn: conn, r: bufio.NewReader(conn)}

	if err := c.login(username, password); err != nil {
		conn.Close()
		return nil, err
	}

	return c, nil
}

func (c *Client) login(username, password string) error {
	if err := writeSentence(c.conn, []string{
		"/login",
		"=name=" + username,
		"=password=" + password,
	}); err != nil {
		return fmt.Errorf("routeros: sending login: %w", err)
	}

	reply, err := c.readReply()
	if err != nil {
		return fmt.Errorf("routeros: reading login reply: %w", err)
	}

	if reply.Status != "done" {
		return fmt.Errorf("routeros: login failed: %s", reply.ErrorMessage())
	}

	return nil
}

// Close closes the underlying TCP connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Reply is the parsed result of running a command: zero or more data
// rows (each a set of attribute=value pairs), a final status word
// ("done" or "trap"), and any trap message attached to it.
type Reply struct {
	Rows   []map[string]string
	Status string
}

// ErrorMessage returns the message attribute of a trap reply, if any.
func (r Reply) ErrorMessage() string {
	for _, row := range r.Rows {
		if msg, ok := row["message"]; ok {
			return msg
		}
	}
	return "unknown error"
}

// Run executes a RouterOS API command (e.g. "/system/resource/print")
// with optional attribute words (e.g. "=interface=ether1") and returns
// the parsed reply.
func (c *Client) Run(command string, args ...string) (Reply, error) {
	words := append([]string{command}, args...)
	if err := writeSentence(c.conn, words); err != nil {
		return Reply{}, fmt.Errorf("routeros: sending command %q: %w", command, err)
	}
	return c.readReply()
}

// readReply reads sentences until a terminating "!done" or "!trap"
// sentence, collecting any "!re" (reply row) sentences along the way.
func (c *Client) readReply() (Reply, error) {
	var reply Reply

	for {
		words, err := readSentence(c.r)
		if err != nil {
			return reply, err
		}
		if len(words) == 0 {
			continue
		}

		switch words[0] {
		case "!re":
			reply.Rows = append(reply.Rows, parseAttributeWords(words[1:]))
		case "!done":
			reply.Status = "done"
			// A !done sentence can itself carry trailing attribute words
			// (e.g. for /login's legacy challenge) — collect them too.
			if len(words) > 1 {
				reply.Rows = append(reply.Rows, parseAttributeWords(words[1:]))
			}
			return reply, nil
		case "!trap":
			reply.Status = "trap"
			reply.Rows = append(reply.Rows, parseAttributeWords(words[1:]))
			return reply, nil
		case "!fatal":
			reply.Status = "fatal"
			return reply, fmt.Errorf("routeros: fatal: %s", strings.Join(words[1:], " "))
		}
	}
}

// parseAttributeWords turns words like "=name=ether1" into a map
// {"name": "ether1"}.
func parseAttributeWords(words []string) map[string]string {
	attrs := make(map[string]string, len(words))
	for _, word := range words {
		if !strings.HasPrefix(word, "=") {
			continue
		}
		rest := word[1:]
		eq := strings.IndexByte(rest, '=')
		if eq < 0 {
			continue
		}
		attrs[rest[:eq]] = rest[eq+1:]
	}
	return attrs
}

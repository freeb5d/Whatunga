package snmp

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

// Value is one decoded varbind value from an SNMP response. Only one
// of the fields is meaningful, selected by Kind.
type Value struct {
	Kind string // "int", "string", "oid", "counter", "gauge", "ticks", "null", "no-such-object", "no-such-instance", "end-of-mib-view"
	Int  int64
	Str  string
}

// String renders a Value for display/JSON purposes regardless of
// its underlying kind.
func (v Value) String() string {
	switch v.Kind {
	case "int", "counter", "gauge", "ticks":
		return fmt.Sprintf("%d", v.Int)
	case "string", "oid":
		return v.Str
	default:
		return v.Kind
	}
}

// Client polls one SNMP agent over UDP using community-based
// authentication (SNMPv2c — the version every consumer/SMB firewall
// this tool targets actually has enabled; SNMPv3's extra security
// layers are out of scope for a monitoring tool on a trusted
// management network).
type Client struct {
	addr      string // "host:161"
	community string
	timeout   time.Duration
}

// NewClient returns a Client for the SNMP agent at addr ("host:161"
// or just "host", in which case the standard SNMP port 161 is used).
func NewClient(addr, community string, timeout time.Duration) *Client {
	if !strings.Contains(addr, ":") {
		addr += ":161"
	}
	return &Client{addr: addr, community: community, timeout: timeout}
}

// Get fetches the values of one or more OIDs in a single request.
// The returned map is keyed by the exact OID string requested.
func (c *Client) Get(oids ...string) (map[string]Value, error) {
	return c.request(tagGetRequest, oids)
}

// GetNext fetches the next OID (lexicographically) after the given
// one, along with its value — the building block for walking a MIB
// subtree (see Walk).
func (c *Client) GetNext(oid string) (nextOID string, value Value, err error) {
	results, err := c.request(tagGetNextRequest, []string{oid})
	if err != nil {
		return "", Value{}, err
	}
	for next, v := range results {
		return next, v, nil // exactly one varbind comes back for a single-OID GetNext
	}
	return "", Value{}, fmt.Errorf("snmp: empty response to GetNext(%s)", oid)
}

// Walk repeatedly issues GetNext starting from baseOID until the
// returned OID is no longer inside the baseOID subtree (or the
// agent reports end-of-MIB-view), returning every (oid, value) pair
// found — the standard SNMP MIB-walking algorithm. This is how
// Whatunga discovers, e.g., every row of a firewall's interface
// table without knowing the row count ahead of time.
func (c *Client) Walk(baseOID string) (map[string]Value, error) {
	results := make(map[string]Value)
	current := baseOID

	for {
		next, value, err := c.GetNext(current)
		if err != nil {
			return results, err
		}
		if value.Kind == "end-of-mib-view" || !strings.HasPrefix(next, baseOID+".") {
			break
		}
		results[next] = value
		current = next

		// Safety valve: a misbehaving agent could otherwise loop forever.
		if len(results) > 10000 {
			return results, fmt.Errorf("snmp: walk of %s exceeded 10000 entries, aborting", baseOID)
		}
	}

	return results, nil
}

// request builds and sends one SNMP message (GetRequest or
// GetNextRequest) containing the given OIDs, and decodes the
// GetResponse into a map of OID -> Value.
func (c *Client) request(pduTag byte, oids []string) (map[string]Value, error) {
	msg, err := buildMessage(c.community, pduTag, rand.Int31(), oids)
	if err != nil {
		return nil, fmt.Errorf("snmp: building request: %w", err)
	}

	conn, err := net.DialTimeout("udp", c.addr, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("snmp: dialing %s: %w", c.addr, err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(c.timeout))

	if _, err := conn.Write(msg); err != nil {
		return nil, fmt.Errorf("snmp: sending request: %w", err)
	}

	buf := make([]byte, 65535)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("snmp: reading response: %w", err)
	}

	return parseResponse(buf[:n])
}

// buildMessage constructs a full SNMPv2c message:
//
//	SEQUENCE {
//	  version    INTEGER (1 = SNMPv2c)
//	  community  OCTET STRING
//	  pdu        [GetRequest|GetNextRequest] {
//	    request-id    INTEGER
//	    error-status  INTEGER (0)
//	    error-index   INTEGER (0)
//	    variable-bindings SEQUENCE OF { name OID, value NULL }
//	  }
//	}
func buildMessage(community string, pduTag byte, requestID int32, oids []string) ([]byte, error) {
	var varbinds []byte
	for _, oid := range oids {
		encodedOID, err := encodeOID(oid)
		if err != nil {
			return nil, err
		}
		varbind := append(encodedOID, encodeTLV(tagNull, nil)...)
		varbinds = append(varbinds, encodeTLV(tagSequence, varbind)...)
	}

	pdu := encodeInteger(int(requestID))
	pdu = append(pdu, encodeInteger(0)...) // error-status
	pdu = append(pdu, encodeInteger(0)...) // error-index
	pdu = append(pdu, encodeTLV(tagSequence, varbinds)...)

	message := encodeInteger(1) // SNMPv2c
	message = append(message, encodeTLV(tagOctetStr, []byte(community))...)
	message = append(message, encodeTLV(pduTag, pdu)...)

	return encodeTLV(tagSequence, message), nil
}

// parseResponse decodes a GetResponse message into a map of
// OID -> Value, in varbind order.
func parseResponse(data []byte) (map[string]Value, error) {
	envelope, err := decodeTLV(data)
	if err != nil {
		return nil, err
	}
	if envelope.tag != tagSequence {
		return nil, fmt.Errorf("snmp: response is not a SEQUENCE (tag 0x%02x)", envelope.tag)
	}

	rest := envelope.content

	// version
	version, err := decodeTLV(rest)
	if err != nil {
		return nil, err
	}
	rest = rest[version.consumed:]

	// community
	community, err := decodeTLV(rest)
	if err != nil {
		return nil, err
	}
	rest = rest[community.consumed:]

	// pdu
	pdu, err := decodeTLV(rest)
	if err != nil {
		return nil, err
	}
	if pdu.tag != tagGetResponse {
		return nil, fmt.Errorf("snmp: expected GetResponse (0x%02x), got 0x%02x", tagGetResponse, pdu.tag)
	}

	pduContent := pdu.content

	// request-id
	reqID, err := decodeTLV(pduContent)
	if err != nil {
		return nil, err
	}
	pduContent = pduContent[reqID.consumed:]

	// error-status
	errStatus, err := decodeTLV(pduContent)
	if err != nil {
		return nil, err
	}
	pduContent = pduContent[errStatus.consumed:]
	if decodeInteger(errStatus.content) != 0 {
		return nil, fmt.Errorf("snmp: agent returned error-status %d", decodeInteger(errStatus.content))
	}

	// error-index
	errIndex, err := decodeTLV(pduContent)
	if err != nil {
		return nil, err
	}
	pduContent = pduContent[errIndex.consumed:]

	// variable-bindings
	varbindsTLV, err := decodeTLV(pduContent)
	if err != nil {
		return nil, err
	}

	return parseVarbinds(varbindsTLV.content)
}

// parseVarbinds decodes a SEQUENCE OF VarBind, each VarBind being
// SEQUENCE { name OID, value ANY }.
func parseVarbinds(data []byte) (map[string]Value, error) {
	results := make(map[string]Value)

	for len(data) > 0 {
		vb, err := decodeTLV(data)
		if err != nil {
			return results, err
		}
		data = data[vb.consumed:]

		nameTLV, err := decodeTLV(vb.content)
		if err != nil {
			return results, err
		}
		oid := decodeOID(nameTLV.content)

		valueTLV, err := decodeTLV(vb.content[nameTLV.consumed:])
		if err != nil {
			return results, err
		}

		results[oid] = decodeValue(valueTLV)
	}

	return results, nil
}

// decodeValue maps a decoded TLV's tag to a Value of the right kind.
func decodeValue(t tlv) Value {
	switch t.tag {
	case tagInteger:
		return Value{Kind: "int", Int: decodeInteger(t.content)}
	case tagOctetStr:
		return Value{Kind: "string", Str: string(t.content)}
	case tagOID:
		return Value{Kind: "oid", Str: decodeOID(t.content)}
	case tagCounter32, tagCounter64:
		return Value{Kind: "counter", Int: decodeInteger(t.content)}
	case tagGauge32:
		return Value{Kind: "gauge", Int: decodeInteger(t.content)}
	case tagTimeTicks:
		return Value{Kind: "ticks", Int: decodeInteger(t.content)}
	case tagNoSuchObject:
		return Value{Kind: "no-such-object"}
	case tagNoSuchInstance:
		return Value{Kind: "no-such-instance"}
	case tagEndOfMibView:
		return Value{Kind: "end-of-mib-view"}
	default:
		return Value{Kind: "null"}
	}
}

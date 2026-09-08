package snmp

import (
	"net"
	"testing"
	"time"
)

// fakeAgentData is a tiny, fixed MIB served by startFakeAgent: one
// scalar (sysDescr) and a two-row fake interface table (ifDescr),
// enough to exercise Get, GetNext, and Walk without needing a real
// SNMP-capable device on hand.
var fakeAgentData = map[string]Value{
	"1.3.6.1.2.1.1.1.0":   {Kind: "string", Str: "Fake Firewall v1.0"},
	"1.3.6.1.2.1.2.2.1.2.1": {Kind: "string", Str: "eth0"},
	"1.3.6.1.2.1.2.2.1.2.2": {Kind: "string", Str: "eth1"},
}

// fakeAgentOrder is fakeAgentData's keys in sorted OID order, since
// GetNext/Walk depend on lexicographic-by-arc OID ordering — Go maps
// don't preserve order, so the fake agent needs an explicit sequence
// to know what "next" means.
var fakeAgentOrder = []string{
	"1.3.6.1.2.1.1.1.0",
	"1.3.6.1.2.1.2.2.1.2.1",
	"1.3.6.1.2.1.2.2.1.2.2",
}

// startFakeAgent starts a minimal in-process UDP server that
// understands just enough SNMPv2c to answer GetRequest and
// GetNextRequest against fakeAgentData, mirroring the same
// "hand-rolled fake server" testing pattern used for the RouterOS
// client in internal/routeros/client_test.go.
func startFakeAgent(t *testing.T) string {
	t.Helper()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting fake SNMP agent: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			response := handleFakeRequest(buf[:n])
			if response != nil {
				conn.WriteTo(response, addr)
			}
		}
	}()

	return conn.LocalAddr().String()
}

// handleFakeRequest decodes just enough of an incoming SNMP message
// to answer a single-OID GetRequest or GetNextRequest against
// fakeAgentData.
func handleFakeRequest(data []byte) []byte {
	envelope, err := decodeTLV(data)
	if err != nil {
		return nil
	}
	rest := envelope.content

	version, err := decodeTLV(rest)
	if err != nil {
		return nil
	}
	rest = rest[version.consumed:]

	community, err := decodeTLV(rest)
	if err != nil {
		return nil
	}
	rest = rest[community.consumed:]

	pdu, err := decodeTLV(rest)
	if err != nil {
		return nil
	}

	pduContent := pdu.content
	reqID, err := decodeTLV(pduContent)
	if err != nil {
		return nil
	}
	pduContent = pduContent[reqID.consumed:]

	errStatus, err := decodeTLV(pduContent)
	if err != nil {
		return nil
	}
	pduContent = pduContent[errStatus.consumed:]

	errIndex, err := decodeTLV(pduContent)
	if err != nil {
		return nil
	}
	pduContent = pduContent[errIndex.consumed:]

	varbindsTLV, err := decodeTLV(pduContent)
	if err != nil {
		return nil
	}

	vb, err := decodeTLV(varbindsTLV.content)
	if err != nil {
		return nil
	}
	nameTLV, err := decodeTLV(vb.content)
	if err != nil {
		return nil
	}
	requestedOID := decodeOID(nameTLV.content)

	var resultOID string
	var resultValue Value

	if pdu.tag == tagGetRequest {
		resultOID = requestedOID
		resultValue = fakeAgentData[requestedOID]
	} else { // GetNextRequest
		resultOID, resultValue = fakeNextAfter(requestedOID)
	}

	return buildFakeResponse(decodeInteger(reqID.content), resultOID, resultValue)
}

func fakeNextAfter(oid string) (string, Value) {
	for i, candidate := range fakeAgentOrder {
		if candidate > oid {
			return candidate, fakeAgentData[candidate]
		}
		_ = i
	}
	return "", Value{Kind: "end-of-mib-view"}
}

func buildFakeResponse(requestID int64, oid string, value Value) []byte {
	var valueTLV []byte
	switch value.Kind {
	case "string":
		valueTLV = encodeTLV(tagOctetStr, []byte(value.Str))
	case "end-of-mib-view":
		valueTLV = encodeTLV(tagEndOfMibView, nil)
	default:
		valueTLV = encodeTLV(tagNull, nil)
	}

	var oidTLV []byte
	if oid != "" {
		oidTLV, _ = encodeOID(oid)
	} else {
		oidTLV, _ = encodeOID("1.3.6.1.2.1.1.1.0") // placeholder, unused when end-of-mib-view
	}

	varbind := encodeTLV(tagSequence, append(oidTLV, valueTLV...))
	varbinds := encodeTLV(tagSequence, varbind)

	pdu := encodeInteger(int(requestID))
	pdu = append(pdu, encodeInteger(0)...)
	pdu = append(pdu, encodeInteger(0)...)
	pdu = append(pdu, varbinds...)

	message := encodeInteger(1)
	message = append(message, encodeTLV(tagOctetStr, []byte("public"))...)
	message = append(message, encodeTLV(tagGetResponse, pdu)...)

	return encodeTLV(tagSequence, message)
}

func TestClient_Get(t *testing.T) {
	addr := startFakeAgent(t)
	client := NewClient(addr, "public", 2*time.Second)

	results, err := client.Get("1.3.6.1.2.1.1.1.0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	value, ok := results["1.3.6.1.2.1.1.1.0"]
	if !ok {
		t.Fatalf("missing expected OID in results: %v", results)
	}
	if value.Str != "Fake Firewall v1.0" {
		t.Errorf("value = %q, want %q", value.Str, "Fake Firewall v1.0")
	}
}

func TestClient_Walk(t *testing.T) {
	addr := startFakeAgent(t)
	client := NewClient(addr, "public", 2*time.Second)

	results, err := client.Walk("1.3.6.1.2.1.2.2.1.2")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2: %v", len(results), results)
	}
	if results["1.3.6.1.2.1.2.2.1.2.1"].Str != "eth0" {
		t.Errorf("row 1 = %q, want %q", results["1.3.6.1.2.1.2.2.1.2.1"].Str, "eth0")
	}
	if results["1.3.6.1.2.1.2.2.1.2.2"].Str != "eth1" {
		t.Errorf("row 2 = %q, want %q", results["1.3.6.1.2.1.2.2.1.2.2"].Str, "eth1")
	}
}

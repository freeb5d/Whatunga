package snmp

import "testing"

func TestEncodeDecodeOID_RoundTrip(t *testing.T) {
	oids := []string{
		"1.3.6.1.2.1.1.1.0",   // sysDescr
		"1.3.6.1.2.1.1.3.0",   // sysUpTime
		"1.3.6.1.2.1.2.2.1.2", // ifDescr
		"1.3.6.1.2.1.2.2.1.10.1",
		"0.0",
		"1.3.6.1.4.1.9999.12345.678", // a large arc value, to exercise multi-byte encoding
	}

	for _, oid := range oids {
		tlvBytes, err := encodeOID(oid)
		if err != nil {
			t.Fatalf("encodeOID(%q): %v", oid, err)
		}

		decoded, err := decodeTLV(tlvBytes)
		if err != nil {
			t.Fatalf("decodeTLV: %v", err)
		}
		if decoded.tag != tagOID {
			t.Fatalf("tag = 0x%02x, want tagOID", decoded.tag)
		}

		got := decodeOID(decoded.content)
		if got != oid {
			t.Errorf("OID round-trip: encoded %q, decoded %q", oid, got)
		}
	}
}

func TestEncodeOID_RejectsTooShort(t *testing.T) {
	if _, err := encodeOID("1"); err == nil {
		t.Fatalf("expected an error for an OID with fewer than two arcs")
	}
}

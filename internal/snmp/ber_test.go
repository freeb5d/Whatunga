package snmp

import "testing"

func TestEncodeDecodeLength_RoundTrip(t *testing.T) {
	lengths := []int{0, 1, 0x7F, 0x80, 0xFF, 0x100, 0xFFFF, 0x10000}

	for _, length := range lengths {
		encoded := encodeLength(length)
		decoded, consumed, err := decodeLength(encoded)
		if err != nil {
			t.Fatalf("decodeLength after encoding %d: %v", length, err)
		}
		if decoded != length {
			t.Errorf("length round-trip: encoded %d, decoded %d", length, decoded)
		}
		if consumed != len(encoded) {
			t.Errorf("length %d: consumed %d bytes, want %d", length, consumed, len(encoded))
		}
	}
}

func TestEncodeDecodeInteger_RoundTrip(t *testing.T) {
	values := []int{0, 1, -1, 127, 128, -128, -129, 255, 256, 65535, -70000, 1 << 20}

	for _, v := range values {
		tlvBytes := encodeInteger(v)

		decoded, err := decodeTLV(tlvBytes)
		if err != nil {
			t.Fatalf("decodeTLV after encoding %d: %v", v, err)
		}
		if decoded.tag != tagInteger {
			t.Fatalf("tag = 0x%02x, want tagInteger", decoded.tag)
		}

		got := decodeInteger(decoded.content)
		if got != int64(v) {
			t.Errorf("integer round-trip: encoded %d, decoded %d", v, got)
		}
	}
}

func TestEncodeTLV_DecodeTLV_RoundTrip(t *testing.T) {
	content := []byte("public")
	encoded := encodeTLV(tagOctetStr, content)

	decoded, err := decodeTLV(encoded)
	if err != nil {
		t.Fatalf("decodeTLV: %v", err)
	}
	if decoded.tag != tagOctetStr {
		t.Errorf("tag = 0x%02x, want 0x%02x", decoded.tag, tagOctetStr)
	}
	if string(decoded.content) != "public" {
		t.Errorf("content = %q, want %q", decoded.content, "public")
	}
	if decoded.consumed != len(encoded) {
		t.Errorf("consumed = %d, want %d", decoded.consumed, len(encoded))
	}
}

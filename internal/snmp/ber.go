// Package snmp implements just enough of SNMPv2c (RFC 3416) — hand
// rolled from the ASN.1 BER spec, the same way internal/routeros
// hand-rolls MikroTik's binary protocol — to poll basic device
// health and interface counters over UDP. SNMP is supported by
// nearly every firewall/network appliance regardless of vendor
// (pfSense, FortiGate, Cisco ASA, Sophos, and more), which makes it
// the most broadly useful protocol for the "several firewall
// models" Whatunga needs to support with one implementation.
//
// Only what Whatunga needs is implemented: GetRequest, GetNextRequest
// (for walking a MIB subtree), and decoding GetResponse PDUs with
// INTEGER, OCTET STRING, OBJECT IDENTIFIER, NULL, Counter32, Gauge32,
// and TimeTicks values. Set requests, SNMPv3, and traps are out of
// scope — this is a monitoring tool, not a full SNMP stack.
package snmp

import "fmt"

// BER tag bytes used by this package.
const (
	tagInteger   = 0x02
	tagOctetStr  = 0x04
	tagNull      = 0x05
	tagOID       = 0x06
	tagSequence  = 0x30
	tagCounter32 = 0x41
	tagGauge32   = 0x42
	tagTimeTicks = 0x43
	tagCounter64 = 0x46

	tagNoSuchObject   = 0x80
	tagNoSuchInstance = 0x81
	tagEndOfMibView   = 0x82

	tagGetRequest     = 0xA0
	tagGetNextRequest = 0xA1
	tagGetResponse    = 0xA2
)

// encodeLength encodes a BER length using the short form (<128) or
// the long form (a length-of-length byte followed by the raw bytes).
func encodeLength(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	// Long form: find the minimum number of bytes needed to hold n.
	var lenBytes []byte
	for v := n; v > 0; v >>= 8 {
		lenBytes = append([]byte{byte(v)}, lenBytes...)
	}
	return append([]byte{byte(0x80 | len(lenBytes))}, lenBytes...)
}

// decodeLength reads a BER length starting at data[0], returning the
// decoded length and the number of bytes the length field itself
// occupied.
func decodeLength(data []byte) (length int, consumed int, err error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("snmp: truncated length")
	}
	first := data[0]
	if first&0x80 == 0 {
		return int(first), 1, nil
	}
	numBytes := int(first &^ 0x80)
	if numBytes == 0 || len(data) < 1+numBytes {
		return 0, 0, fmt.Errorf("snmp: truncated long-form length")
	}
	length = 0
	for i := 0; i < numBytes; i++ {
		length = length<<8 | int(data[1+i])
	}
	return length, 1 + numBytes, nil
}

// encodeTLV wraps content in a BER tag+length+value envelope.
func encodeTLV(tag byte, content []byte) []byte {
	out := append([]byte{tag}, encodeLength(len(content))...)
	return append(out, content...)
}

// tlv is one decoded BER Tag-Length-Value triple, plus how many bytes
// of the input it consumed (tag + length + value), so callers can
// advance through a sequence.
type tlv struct {
	tag     byte
	content []byte
	consumed int
}

// decodeTLV reads one BER TLV starting at data[0].
func decodeTLV(data []byte) (tlv, error) {
	if len(data) < 2 {
		return tlv{}, fmt.Errorf("snmp: truncated TLV")
	}
	tag := data[0]
	length, lenSize, err := decodeLength(data[1:])
	if err != nil {
		return tlv{}, err
	}
	start := 1 + lenSize
	if len(data) < start+length {
		return tlv{}, fmt.Errorf("snmp: truncated TLV content")
	}
	return tlv{
		tag:      tag,
		content:  data[start : start+length],
		consumed: start + length,
	}, nil
}

// encodeInteger encodes n as a minimal two's-complement BER INTEGER.
func encodeInteger(n int) []byte {
	if n == 0 {
		return encodeTLV(tagInteger, []byte{0})
	}
	var bytes []byte
	v := n
	negative := n < 0
	for {
		bytes = append([]byte{byte(v)}, bytes...)
		v >>= 8
		if (!negative && v == 0 && bytes[0]&0x80 == 0) || (negative && v == -1 && bytes[0]&0x80 != 0) {
			break
		}
	}
	return encodeTLV(tagInteger, bytes)
}

// decodeInteger decodes a two's-complement BER INTEGER.
func decodeInteger(content []byte) int64 {
	if len(content) == 0 {
		return 0
	}
	v := int64(int8(content[0])) // sign-extend the first byte
	for _, b := range content[1:] {
		v = v<<8 | int64(b)
	}
	return v
}

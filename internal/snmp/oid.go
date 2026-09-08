package snmp

import (
	"fmt"
	"strconv"
	"strings"
)

// encodeOID encodes a dotted OID string like "1.3.6.1.2.1.1.1.0"
// using the standard OID encoding: the first two arcs are combined
// into a single byte as (arc0*40 + arc1), and every arc after that
// is encoded in base-128 with the high bit set on all but the last
// byte of each arc (a "continuation" bit, much like UTF-8's
// multi-byte scheme).
func encodeOID(oid string) ([]byte, error) {
	parts := strings.Split(oid, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("snmp: OID %q needs at least two arcs", oid)
	}

	arcs := make([]uint64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("snmp: invalid OID arc %q: %w", p, err)
		}
		arcs[i] = v
	}

	var out []byte
	out = append(out, byte(arcs[0]*40+arcs[1]))

	for _, arc := range arcs[2:] {
		out = append(out, encodeOIDArc(arc)...)
	}

	return encodeTLV(tagOID, out), nil
}

// encodeOIDArc base-128-encodes a single arc value with continuation
// bits set on every byte except the last.
func encodeOIDArc(arc uint64) []byte {
	if arc == 0 {
		return []byte{0}
	}
	var bytes []byte
	for v := arc; v > 0; v >>= 7 {
		bytes = append([]byte{byte(v & 0x7F)}, bytes...)
	}
	for i := 0; i < len(bytes)-1; i++ {
		bytes[i] |= 0x80
	}
	return bytes
}

// decodeOID decodes BER-encoded OID content back into dotted form.
func decodeOID(content []byte) string {
	if len(content) == 0 {
		return ""
	}

	first := content[0]
	arcs := []uint64{uint64(first) / 40, uint64(first) % 40}

	var current uint64
	for _, b := range content[1:] {
		current = current<<7 | uint64(b&0x7F)
		if b&0x80 == 0 {
			arcs = append(arcs, current)
			current = 0
		}
	}

	parts := make([]string, len(arcs))
	for i, a := range arcs {
		parts[i] = strconv.FormatUint(a, 10)
	}
	return strings.Join(parts, ".")
}

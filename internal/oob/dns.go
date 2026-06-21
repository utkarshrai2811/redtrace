package oob

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

const (
	dnsTypeA     = 1
	dnsTypeNS    = 2
	dnsTypeAAAA  = 28
	dnsTypeTXT   = 16
	dnsTypeMX    = 15
	dnsTypeCNAME = 5
)

// parseDNSQuery extracts the header ID, the first question's lowercased QNAME,
// its QTYPE, and the offset just past the question section. It returns ok=false
// on any malformed or truncated input — the listener handles untrusted packets,
// so every length is bounds-checked.
func parseDNSQuery(msg []byte) (id, qtype uint16, qname string, qend int, ok bool) {
	if len(msg) < 12 {
		return 0, 0, "", 0, false
	}
	id = binary.BigEndian.Uint16(msg[0:2])
	if binary.BigEndian.Uint16(msg[4:6]) < 1 { // QDCOUNT
		return id, 0, "", 0, false
	}
	off := 12
	var labels []string
	for {
		if off >= len(msg) {
			return id, 0, "", 0, false
		}
		l := int(msg[off])
		off++
		if l == 0 {
			break
		}
		// A question name is never compressed; reject pointers and over-long labels.
		if l > 63 || off+l > len(msg) {
			return id, 0, "", 0, false
		}
		labels = append(labels, string(msg[off:off+l]))
		off += l
		if len(labels) > 127 {
			return id, 0, "", 0, false
		}
	}
	if off+4 > len(msg) { // QTYPE(2) + QCLASS(2)
		return id, 0, "", 0, false
	}
	qtype = binary.BigEndian.Uint16(msg[off : off+2])
	off += 4
	return id, qtype, strings.ToLower(strings.Join(labels, ".")), off, true
}

// buildDNSResponse echoes the query's question and, for an A query when ip is a
// valid IPv4, appends a single A answer pointing at ip.
func buildDNSResponse(query []byte, qend int, qtype uint16, ip net.IP) []byte {
	out := make([]byte, qend)
	copy(out, query[:qend])
	// Flags: QR=1, AA=1, copy the request's RD bit; RCODE=0.
	out[2] = 0x80 | 0x04 | (query[2] & 0x01)
	out[3] = 0
	binary.BigEndian.PutUint16(out[4:6], 1) // QDCOUNT
	binary.BigEndian.PutUint16(out[6:8], 0) // ANCOUNT (set below if we answer)
	binary.BigEndian.PutUint16(out[8:10], 0)
	binary.BigEndian.PutUint16(out[10:12], 0)

	if qtype == dnsTypeA {
		if ip4 := ip.To4(); ip4 != nil {
			out = append(out,
				0xC0, 0x0C, // NAME: pointer to the question name
				0, dnsTypeA,
				0, 1, // CLASS IN
				0, 0, 0, 30, // TTL 30s
				0, 4, // RDLENGTH
				ip4[0], ip4[1], ip4[2], ip4[3],
			)
			binary.BigEndian.PutUint16(out[6:8], 1) // ANCOUNT = 1
		}
	}
	return out
}

func dnsTypeName(t uint16) string {
	switch t {
	case dnsTypeA:
		return "A"
	case dnsTypeAAAA:
		return "AAAA"
	case dnsTypeNS:
		return "NS"
	case dnsTypeTXT:
		return "TXT"
	case dnsTypeMX:
		return "MX"
	case dnsTypeCNAME:
		return "CNAME"
	default:
		return fmt.Sprintf("TYPE%d", t)
	}
}

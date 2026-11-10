// Package conntrack reads the kernel connection table (nf_conntrack) to show
// the network connections currently made by VPN clients.
package conntrack

import (
	"errors"
	"net"
	"sort"
	"time"
)

// ErrUnsupported is returned on systems without nf_conntrack (e.g. Windows/macOS).
var ErrUnsupported = errors.New("connection table is only available on Linux")

// Conn is a single connection initiated by a client.
type Conn struct {
	Proto    string     `json:"proto"` // tcp, udp, icmp, ...
	SrcIP    string     `json:"src_ip"`
	SrcPort  uint16     `json:"src_port"`
	DstIP    string     `json:"dst_ip"`
	DstPort  uint16     `json:"dst_port"`
	Sent     uint64     `json:"sent"`     // bytes from client to destination
	Received uint64     `json:"received"` // bytes from destination to client
	Packets  uint64     `json:"packets"`
	State    string     `json:"state,omitempty"` // TCP state
	Started  *time.Time `json:"started,omitempty"`
	Expires  int        `json:"expires"` // seconds until the kernel removes the entry
}

// ForSource returns connections originating from ip, largest first.
func ForSource(ip net.IP) ([]Conn, error) {
	all, err := list()
	if err != nil {
		return nil, err
	}
	out := make([]Conn, 0, 32)
	for _, c := range all {
		if net.ParseIP(c.SrcIP).Equal(ip) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Sent+out[i].Received > out[j].Sent+out[j].Received
	})
	return out, nil
}

func protoName(p uint8) string {
	switch p {
	case 1:
		return "icmp"
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 58:
		return "icmpv6"
	case 132:
		return "sctp"
	}
	return "other"
}

var tcpStates = []string{"", "SYN_SENT", "SYN_RECV", "ESTABLISHED", "FIN_WAIT", "CLOSE_WAIT", "LAST_ACK", "TIME_WAIT", "CLOSE", "SYN_SENT2"}

func tcpState(s uint8) string {
	if int(s) < len(tcpStates) {
		return tcpStates[s]
	}
	return ""
}

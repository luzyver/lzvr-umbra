// Package conntrack membaca tabel koneksi kernel (nf_conntrack) untuk melihat
// koneksi jaringan yang sedang dibuat oleh klien VPN.
package conntrack

import (
	"errors"
	"net"
	"sort"
	"time"
)

// ErrUnsupported dikembalikan di sistem tanpa nf_conntrack (mis. Windows/macOS).
var ErrUnsupported = errors.New("tabel koneksi hanya tersedia di Linux")

// Conn adalah satu koneksi yang dimulai oleh klien.
type Conn struct {
	Proto    string     `json:"proto"` // tcp, udp, icmp, ...
	SrcIP    string     `json:"src_ip"`
	SrcPort  uint16     `json:"src_port"`
	DstIP    string     `json:"dst_ip"`
	DstPort  uint16     `json:"dst_port"`
	Sent     uint64     `json:"sent"`     // byte dari klien ke tujuan
	Received uint64     `json:"received"` // byte dari tujuan ke klien
	Packets  uint64     `json:"packets"`
	State    string     `json:"state,omitempty"` // status TCP
	Started  *time.Time `json:"started,omitempty"`
	Expires  int        `json:"expires"` // detik sampai entri dihapus kernel
}

// ForSource mengembalikan koneksi yang berasal dari ip, terbesar dulu.
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
	return "lainnya"
}

var tcpStates = []string{"", "SYN_SENT", "SYN_RECV", "ESTABLISHED", "FIN_WAIT", "CLOSE_WAIT", "LAST_ACK", "TIME_WAIT", "CLOSE", "SYN_SENT2"}

func tcpState(s uint8) string {
	if int(s) < len(tcpStates) {
		return tcpStates[s]
	}
	return ""
}

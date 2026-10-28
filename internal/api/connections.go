package api

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/luzyver/lzvr-umbra/internal/conntrack"
)

type connectionsResp struct {
	Conns []conntrack.Conn `json:"conns"`
	// Accounting false bila kernel tidak menghitung byte (nf_conntrack_acct=0).
	Accounting  bool   `json:"accounting"`
	Unsupported string `json:"unsupported,omitempty"`
}

// handleClientConnections menampilkan koneksi yang sedang dibuat klien
// (dari tabel conntrack kernel).
func (s *Server) handleClientConnections(w http.ResponseWriter, r *http.Request) {
	c, ok := s.clientOr404(w, r)
	if !ok {
		return
	}
	ip := net.ParseIP(strings.SplitN(c.Address, "/", 2)[0])
	if ip == nil {
		writeError(w, http.StatusBadRequest, "alamat klien tidak valid")
		return
	}

	var conns []conntrack.Conn
	var err error
	if s.cfg.FakeWG {
		conns = fakeConns(ip)
	} else {
		conns, err = conntrack.ForSource(ip)
	}
	resp := connectionsResp{Conns: conns}
	if errors.Is(err, conntrack.ErrUnsupported) {
		resp.Unsupported = err.Error()
	} else if err != nil {
		internalError(w, "baca tabel koneksi", err)
		return
	}
	if resp.Conns == nil {
		resp.Conns = []conntrack.Conn{}
	}
	resp.Accounting = len(resp.Conns) == 0
	for _, cn := range resp.Conns {
		if cn.Sent+cn.Received > 0 {
			resp.Accounting = true
			break
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// fakeConns memberi contoh koneksi untuk mode dev.
func fakeConns(ip net.IP) []conntrack.Conn {
	now := time.Now()
	mk := func(proto, dst string, port uint16, sent, recv uint64, state string, age time.Duration) conntrack.Conn {
		start := now.Add(-age)
		return conntrack.Conn{
			Proto: proto, SrcIP: ip.String(), SrcPort: uint16(40000 + age.Seconds()),
			DstIP: dst, DstPort: port, Sent: sent, Received: recv,
			Packets: (sent + recv) / 900, State: state, Started: &start, Expires: 300,
		}
	}
	return []conntrack.Conn{
		mk("tcp", "142.250.4.190", 443, 1_200_000, 48_500_000, "ESTABLISHED", 9*time.Minute),
		mk("udp", "142.250.4.190", 443, 340_000, 12_800_000, "", 4*time.Minute),
		mk("tcp", "157.240.13.35", 443, 85_000, 2_300_000, "ESTABLISHED", 2*time.Minute),
		mk("tcp", "140.82.112.4", 443, 12_000, 64_000, "TIME_WAIT", 40*time.Second),
		mk("udp", "10.8.0.1", 53, 900, 2_400, "", 10*time.Second),
	}
}

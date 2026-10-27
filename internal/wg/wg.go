// Package wg mengatur interface WireGuard. Perubahan langsung dikirim ke kernel
// (tanpa tombol "Apply Config"), lalu salinan wg0.conf ditulis sebagai cadangan.
package wg

import (
	"time"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

// Peer adalah klien yang aktif dan harus ada di interface.
type Peer struct {
	PublicKey    string
	PresharedKey string
	Address      string // IP tunggal klien, dipasang sebagai /32
	Keepalive    int
}

type PeerStat struct {
	PublicKey     string    `json:"public_key"`
	Endpoint      string    `json:"endpoint"`
	LastHandshake time.Time `json:"last_handshake"`
	RxBytes       int64     `json:"rx_bytes"`
	TxBytes       int64     `json:"tx_bytes"`
}

type Backend interface {
	// EnsureInterface membuat interface bila belum ada, lalu memasang alamat, MTU, dan menyalakannya.
	EnsureInterface(cfg store.ServerConfig) error
	// Sync menyamakan daftar peer di kernel dengan daftar dari database.
	Sync(cfg store.ServerConfig, peers []Peer) error
	Stats(iface string) ([]PeerStat, error)
}

// PeersFromClients mengambil klien yang aktif saja.
func PeersFromClients(clients []store.Client) []Peer {
	out := make([]Peer, 0, len(clients))
	for _, c := range clients {
		if !c.Enabled {
			continue
		}
		out = append(out, Peer{
			PublicKey:    c.PublicKey,
			PresharedKey: c.PresharedKey,
			Address:      c.Address,
		})
	}
	return out
}

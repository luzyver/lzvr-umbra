// Package wg manages the WireGuard interface. Changes are sent straight to the backend
// (no "Apply Config" button), and a copy of wg0.conf is written as a backup.
package wg

import (
	"time"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

// Peer is an active client that must exist on the interface.
type Peer struct {
	PublicKey    string
	PresharedKey string
	Address      string // the client's single IP, installed as /32
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
	// Name returns the active backend, or pending if not ready yet.
	Name() string
	// Close releases the resources held by the process.
	Close() error
	// EnsureInterface creates the interface if missing, then sets its address and MTU and brings it up.
	EnsureInterface(cfg store.ServerConfig) error
	// Sync reconciles the peer list with the list from the database.
	Sync(cfg store.ServerConfig, peers []Peer) error
	Stats(iface string) ([]PeerStat, error)
}

// PeersFromClients picks enabled clients only.
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

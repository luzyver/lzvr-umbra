package wg

import (
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

// Fake meniru WireGuard untuk pengembangan di Windows/macOS: menyimpan peer di memori
// dan mengarang statistik supaya dashboard bisa dicoba.
type Fake struct {
	mu    sync.Mutex
	peers map[string]*PeerStat
}

func NewFake() *Fake { return &Fake{peers: map[string]*PeerStat{}} }

func (*Fake) Name() string { return "fake" }
func (*Fake) Close() error { return nil }

func (f *Fake) EnsureInterface(cfg store.ServerConfig) error {
	slog.Info("fake wg: interface siap", "iface", cfg.Interface, "address", cfg.Address)
	return nil
}

func (f *Fake) Sync(cfg store.ServerConfig, peers []Peer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	next := map[string]*PeerStat{}
	for _, p := range peers {
		if old, ok := f.peers[p.PublicKey]; ok {
			next[p.PublicKey] = old
		} else {
			next[p.PublicKey] = &PeerStat{PublicKey: p.PublicKey}
		}
	}
	f.peers = next
	slog.Info("fake wg: sync", "iface", cfg.Interface, "peers", len(peers))
	return nil
}

func (f *Fake) Stats(string) ([]PeerStat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]PeerStat, 0, len(f.peers))
	for _, p := range f.peers {
		// separuh peer "online" dengan trafik yang terus bertambah
		if p.PublicKey[0]%2 == 0 {
			p.LastHandshake = time.Now().Add(-time.Duration(rand.IntN(90)) * time.Second)
			p.Endpoint = "203.0.113.7:51820"
			p.RxBytes += int64(rand.IntN(200_000))
			p.TxBytes += int64(rand.IntN(800_000))
		}
		out = append(out, *p)
	}
	return out, nil
}

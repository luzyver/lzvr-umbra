// Package importer reads WireGuard backups without modifying the database.
package importer

import (
	"github.com/luzyver/lzvr-umbra/internal/store"
	"net/netip"
	"strings"
	"time"
)

type Candidate struct {
	Name          string    `json:"name"`
	PrivateKey    string    `json:"-"`
	PublicKey     string    `json:"public_key"`
	PresharedKey  string    `json:"-"`
	Address       string    `json:"address"`
	AllowedIPs    []string  `json:"-"`
	DNS           []string  `json:"-"`
	MTU           int       `json:"-"`
	Keepalive     int       `json:"-"`
	Enabled       bool      `json:"enabled"`
	Reason        string    `json:"reason,omitempty"`
	HasPrivateKey bool      `json:"has_private_key"`
	CreatedAt     time.Time `json:"-"`
	UpdatedAt     time.Time `json:"-"`
}

type Source struct {
	Server           store.ServerConfig
	Clients          []Candidate
	DefaultDNS       []string
	DefaultMTU       int
	DefaultKeepalive int
}

func (c Candidate) Client() store.Client {
	return store.Client{Name: c.Name, PrivateKey: c.PrivateKey, HasPrivateKey: c.PrivateKey != "", PublicKey: c.PublicKey, PresharedKey: c.PresharedKey, Address: c.Address, AllowedIPs: c.AllowedIPs, DNS: c.DNS, MTU: c.MTU, Keepalive: c.Keepalive, Enabled: c.Enabled, Adblock: true, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func Check(in, existing []Candidate, subnet string) []Candidate {
	p, err := netip.ParsePrefix(subnet)
	keys, ips := map[string]bool{}, map[string]bool{}
	for _, c := range existing {
		keys[c.PublicKey] = true
		ips[c.Address] = true
	}
	out := make([]Candidate, len(in))
	for i, c := range in {
		a, e := netip.ParseAddr(c.Address)
		switch {
		case c.Reason != "":
			// already rejected earlier (e.g. invalid key)
		case err != nil || e != nil || !p.Contains(a) || a == p.Addr() || a == p.Masked().Addr() || !p.Contains(a.Next()):
			c.Reason = "address outside server subnet"
		case keys[c.PublicKey]:
			c.Reason = "public key already exists"
		case ips[c.Address]:
			c.Reason = "address already exists"
		}
		if c.Reason == "" {
			keys[c.PublicKey] = true
			ips[c.Address] = true
		}
		c.Name = strings.TrimSpace(c.Name)
		out[i] = c
	}
	return out
}

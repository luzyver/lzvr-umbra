package store

import "time"

// ServerConfig adalah konfigurasi interface WireGuard di server.
type ServerConfig struct {
	Interface    string `json:"interface"`     // mis. wg0
	Address      string `json:"address"`       // CIDR server, mis. 10.8.0.1/24
	ListenPort   int    `json:"listen_port"`   // UDP, mis. 51820
	EndpointHost string `json:"endpoint_host"` // domain/IP publik untuk klien
	EndpointPort int    `json:"endpoint_port"` // port publik (bisa beda kalau di balik NAT)
	MTU          int    `json:"mtu"`
	OutInterface string `json:"out_interface"` // interface keluar untuk MASQUERADE, mis. enp3s0
	PrivateKey   string `json:"private_key,omitempty"`
	PublicKey    string `json:"public_key"`
	Initialized  bool   `json:"initialized"`
}

// ClientDefaults dipakai saat membuat klien baru.
type ClientDefaults struct {
	DNS        []string `json:"dns"`
	AllowedIPs []string `json:"allowed_ips"`
	Keepalive  int      `json:"keepalive"`
	MTU        int      `json:"mtu"`
}

// FirewallState menyimpan aturan firewall yang sudah dikonfirmasi.
type FirewallState struct {
	Applied  bool `json:"applied"`
	MSSClamp bool `json:"mss_clamp"`
}

type Client struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	PrivateKey   string    `json:"-"`
	PublicKey    string    `json:"public_key"`
	PresharedKey string    `json:"-"`
	Address      string    `json:"address"` // IP tunggal, mis. 10.8.0.2
	AllowedIPs   []string  `json:"allowed_ips"`
	DNS          []string  `json:"dns"`
	MTU          int       `json:"mtu"`
	Keepalive    int       `json:"keepalive"`
	Enabled      bool      `json:"enabled"`
	Adblock      bool      `json:"adblock"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
}

const (
	KeyServer   = "server"
	KeyDefaults = "client_defaults"
	KeyFirewall = "firewall"
)

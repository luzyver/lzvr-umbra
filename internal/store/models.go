package store

import "time"

// ServerConfig is the WireGuard interface configuration on the server.
type ServerConfig struct {
	Interface    string `json:"interface"`     // e.g. wg0
	Address      string `json:"address"`       // server CIDR, e.g. 10.8.0.1/24
	ListenPort   int    `json:"listen_port"`   // UDP, e.g. 51820
	EndpointHost string `json:"endpoint_host"` // public domain/IP for clients
	EndpointPort int    `json:"endpoint_port"` // public port (may differ when behind NAT)
	MTU          int    `json:"mtu"`
	OutInterface string `json:"out_interface"` // outbound interface for MASQUERADE, e.g. enp3s0
	PrivateKey   string `json:"private_key,omitempty"`
	PublicKey    string `json:"public_key"`
	Initialized  bool   `json:"initialized"`
}

// ClientDefaults are used when creating a new client.
type ClientDefaults struct {
	DNS        []string `json:"dns"`
	AllowedIPs []string `json:"allowed_ips"`
	Keepalive  int      `json:"keepalive"`
	MTU        int      `json:"mtu"`
}

// FirewallState holds the confirmed firewall rules.
type FirewallState struct {
	Applied  bool `json:"applied"`
	MSSClamp bool `json:"mss_clamp"`
}

type Client struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	PrivateKey    string    `json:"-"`
	HasPrivateKey bool      `json:"has_private_key"`
	PublicKey     string    `json:"public_key"`
	PresharedKey  string    `json:"-"`
	Address       string    `json:"address"` // single IP, e.g. 10.8.0.2
	AllowedIPs    []string  `json:"allowed_ips"`
	DNS           []string  `json:"dns"`
	MTU           int       `json:"mtu"`
	Keepalive     int       `json:"keepalive"`
	Enabled       bool      `json:"enabled"`
	Adblock       bool      `json:"adblock"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
	TOTPSecret   string
	LastTOTPStep int64
}

const (
	KeyServer   = "server"
	KeyDefaults = "client_defaults"
	KeyFirewall = "firewall"
	KeyAdGuard  = "adguard"
)

// AdGuardConn is the connection to AdGuard Home configured from the panel.
// Managed means AdGuard was installed and set up by the panel itself.
type AdGuardConn struct {
	URL     string `json:"url"`
	User    string `json:"user"`
	Pass    string `json:"pass"`
	Managed bool   `json:"managed"`
}

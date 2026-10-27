package wg

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

// RenderClientConfig membuat isi file .conf untuk diimpor di aplikasi WireGuard klien.
func RenderClientConfig(srv store.ServerConfig, c store.Client) string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "# %s\n", oneLine(c.Name))
	fmt.Fprintf(&b, "PrivateKey = %s\n", c.PrivateKey)
	fmt.Fprintf(&b, "Address = %s/32\n", c.Address)
	if len(c.DNS) > 0 {
		fmt.Fprintf(&b, "DNS = %s\n", strings.Join(c.DNS, ", "))
	}
	if c.MTU > 0 {
		fmt.Fprintf(&b, "MTU = %d\n", c.MTU)
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", srv.PublicKey)
	if c.PresharedKey != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", c.PresharedKey)
	}
	fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(c.AllowedIPs, ", "))
	fmt.Fprintf(&b, "Endpoint = %s\n", net.JoinHostPort(srv.EndpointHost, strconv.Itoa(endpointPort(srv))))
	if c.Keepalive > 0 {
		fmt.Fprintf(&b, "PersistentKeepalive = %d\n", c.Keepalive)
	}
	return b.String()
}

// RenderServerConfig membuat wg0.conf cadangan (format wg-quick) dari isi database.
// Panel tidak membaca file ini; gunanya agar server tetap bisa dipulihkan dengan wg-quick.
func RenderServerConfig(srv store.ServerConfig, clients []store.Client) string {
	var b strings.Builder
	b.WriteString("# Dibuat otomatis oleh umbra. Jangan diedit: perubahan akan ditimpa.\n")
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "Address = %s\n", srv.Address)
	fmt.Fprintf(&b, "ListenPort = %d\n", srv.ListenPort)
	fmt.Fprintf(&b, "PrivateKey = %s\n", srv.PrivateKey)
	if srv.MTU > 0 {
		fmt.Fprintf(&b, "MTU = %d\n", srv.MTU)
	}
	for _, c := range clients {
		if !c.Enabled {
			continue
		}
		b.WriteString("\n[Peer]\n")
		fmt.Fprintf(&b, "# %s\n", oneLine(c.Name))
		fmt.Fprintf(&b, "PublicKey = %s\n", c.PublicKey)
		if c.PresharedKey != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", c.PresharedKey)
		}
		fmt.Fprintf(&b, "AllowedIPs = %s/32\n", c.Address)
	}
	return b.String()
}

func endpointPort(srv store.ServerConfig) int {
	if srv.EndpointPort > 0 {
		return srv.EndpointPort
	}
	return srv.ListenPort
}

// oneLine mencegah nama klien menyisipkan baris baru ke file konfigurasi.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
}

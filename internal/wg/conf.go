package wg

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

// RenderClientConfig produces the .conf contents to import into the client's WireGuard app.
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

// RenderServerConfig produces a backup wg0.conf (wg-quick format) from the database contents.
// The panel does not read this file; it exists so the server can still be restored with wg-quick.
func RenderServerConfig(srv store.ServerConfig, clients []store.Client) string {
	var b strings.Builder
	b.WriteString("# Generated automatically by umbra. Do not edit: changes will be overwritten.\n")
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

// oneLine prevents a client name from injecting newlines into the config file.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
}

// Package system checks whether the server is ready for WireGuard (used by the setup wizard).
package system

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Item struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok | warn | fail
	Detail string `json:"detail"`
}

type Report struct {
	Items        []Item   `json:"items"`
	DefaultIface string   `json:"default_iface"`
	LocalIPs     []string `json:"local_ips"`
	PublicIP     string   `json:"public_ip"`
	BehindNAT    bool     `json:"behind_nat"`
}

func Check(ctx context.Context, listenPort int) Report {
	var r Report
	add := func(id, label, status, detail string) {
		r.Items = append(r.Items, Item{ID: id, Label: label, Status: status, Detail: detail})
	}

	if runtime.GOOS != "linux" {
		add("os", "Operating system", "warn", "Not Linux ("+runtime.GOOS+"): the panel is running in simulated mode")
	}

	if _, err := os.Stat("/sys/module/wireguard"); err == nil {
		add("module", "WireGuard kernel module", "ok", "Already loaded")
	} else {
		add("module", "WireGuard kernel module", "warn", "Module not detected. Auto mode uses userspace wireguard-go if the kernel does not support WireGuard; this needs /dev/net/tun and CAP_NET_ADMIN. Kernel mode requires kernel support.")
	}

	if b, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil && strings.TrimSpace(string(b)) == "1" {
		add("forward", "IP forwarding", "ok", "Enabled")
	} else {
		add("forward", "IP forwarding", "fail", "Disabled. The installer enables net.ipv4.ip_forward=1")
	}

	if _, err := exec.LookPath("nft"); err == nil {
		add("nft", "nftables", "ok", "nft command is available")
	} else {
		add("nft", "nftables", "fail", "nft command not found. Install the nftables package")
	}

	if listenPort > 0 {
		if conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", listenPort)); err == nil {
			conn.Close()
			add("port", fmt.Sprintf("Port UDP %d", listenPort), "ok", "Not used by any other program")
		} else {
			add("port", fmt.Sprintf("Port UDP %d", listenPort), "warn",
				"Already in use (possibly by an old WireGuard/wg-easy): "+err.Error())
		}
	}

	if st := ufwStatus(); st != "" {
		status := "ok"
		if st == "active, forward DROP" {
			status = "warn"
		}
		add("ufw", "UFW", status, ufwDetail(st))
	}

	r.DefaultIface = defaultIface()
	if r.DefaultIface != "" {
		add("route", "Internet interface", "ok", r.DefaultIface)
	} else {
		add("route", "Internet interface", "warn", "Default route not found, enter it manually")
	}

	r.LocalIPs = localIPs()
	r.PublicIP = publicIP(ctx)
	if r.PublicIP != "" {
		r.BehindNAT = true
		for _, ip := range r.LocalIPs {
			if ip == r.PublicIP {
				r.BehindNAT = false
			}
		}
		if r.BehindNAT {
			add("nat", "Public IP", "warn", fmt.Sprintf(
				"Server is behind NAT (public IP %s, local IP %s). Make sure the UDP port is forwarded to this server.",
				r.PublicIP, strings.Join(r.LocalIPs, ", ")))
		} else {
			add("nat", "Public IP", "ok", r.PublicIP)
		}
	} else {
		add("nat", "Public IP", "warn", "Cannot detect the public IP")
	}
	return r
}

func ufwStatus() string {
	conf, err := os.ReadFile("/etc/ufw/ufw.conf")
	if err != nil {
		return ""
	}
	if !strings.Contains(string(conf), "ENABLED=yes") {
		return "inactive"
	}
	def, _ := os.ReadFile("/etc/default/ufw")
	if strings.Contains(string(def), `DEFAULT_FORWARD_POLICY="DROP"`) {
		return "active, forward DROP"
	}
	return "active"
}

func ufwDetail(st string) string {
	switch st {
	case "inactive":
		return "Inactive"
	case "active, forward DROP":
		return "Active with forward DROP. The installer adds 'ufw route allow in on wg0' and opens the UDP port"
	}
	return "Active"
}

// defaultIface reads /proc/net/route and picks the interface whose destination is 0.0.0.0.
func defaultIface() string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // header
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) > 2 && fields[1] == "00000000" {
			if mask, err := hex.DecodeString(fields[7]); err == nil && len(mask) == 4 && mask[0] == 0 {
				return fields[0]
			}
		}
	}
	return ""
}

func localIPs() []string {
	out := []string{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() {
			out = append(out, n.IP.String())
		}
	}
	return out
}

func publicIP(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	ip := net.ParseIP(strings.TrimSpace(string(b)))
	if ip == nil {
		return ""
	}
	return ip.String()
}

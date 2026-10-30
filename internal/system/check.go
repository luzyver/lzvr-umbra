// Package system memeriksa kesiapan server untuk WireGuard (dipakai wizard setup).
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
		add("os", "Sistem operasi", "warn", "Bukan Linux ("+runtime.GOOS+"): panel berjalan dalam mode tiruan")
	}

	if _, err := os.Stat("/sys/module/wireguard"); err == nil {
		add("module", "Modul kernel WireGuard", "ok", "Sudah dimuat")
	} else {
		add("module", "Modul kernel WireGuard", "warn", "Modul tidak terdeteksi. Mode auto memakai wireguard-go userspace bila kernel tidak mendukung WireGuard; perlu /dev/net/tun dan CAP_NET_ADMIN. Mode kernel memerlukan dukungan kernel.")
	}

	if b, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil && strings.TrimSpace(string(b)) == "1" {
		add("forward", "IP forwarding", "ok", "Aktif")
	} else {
		add("forward", "IP forwarding", "fail", "Mati. Installer mengaktifkan net.ipv4.ip_forward=1")
	}

	if _, err := exec.LookPath("nft"); err == nil {
		add("nft", "nftables", "ok", "Perintah nft tersedia")
	} else {
		add("nft", "nftables", "fail", "Perintah nft tidak ada. Pasang paket nftables")
	}

	if listenPort > 0 {
		if conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", listenPort)); err == nil {
			conn.Close()
			add("port", fmt.Sprintf("Port UDP %d", listenPort), "ok", "Belum dipakai program lain")
		} else {
			add("port", fmt.Sprintf("Port UDP %d", listenPort), "warn",
				"Sudah dipakai (mungkin oleh WireGuard/wg-easy lama): "+err.Error())
		}
	}

	if st := ufwStatus(); st != "" {
		status := "ok"
		if st == "aktif, forward DROP" {
			status = "warn"
		}
		add("ufw", "UFW", status, ufwDetail(st))
	}

	r.DefaultIface = defaultIface()
	if r.DefaultIface != "" {
		add("route", "Interface internet", "ok", r.DefaultIface)
	} else {
		add("route", "Interface internet", "warn", "Default route tidak ditemukan, isi manual")
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
			add("nat", "IP publik", "warn", fmt.Sprintf(
				"Server di balik NAT (IP publik %s, IP lokal %s). Pastikan port UDP diteruskan ke server ini.",
				r.PublicIP, strings.Join(r.LocalIPs, ", ")))
		} else {
			add("nat", "IP publik", "ok", r.PublicIP)
		}
	} else {
		add("nat", "IP publik", "warn", "Tidak bisa mendeteksi IP publik")
	}
	return r
}

func ufwStatus() string {
	conf, err := os.ReadFile("/etc/ufw/ufw.conf")
	if err != nil {
		return ""
	}
	if !strings.Contains(string(conf), "ENABLED=yes") {
		return "mati"
	}
	def, _ := os.ReadFile("/etc/default/ufw")
	if strings.Contains(string(def), `DEFAULT_FORWARD_POLICY="DROP"`) {
		return "aktif, forward DROP"
	}
	return "aktif"
}

func ufwDetail(st string) string {
	switch st {
	case "mati":
		return "Tidak aktif"
	case "aktif, forward DROP":
		return "Aktif dengan forward DROP. Installer menambahkan 'ufw route allow in on wg0' dan membuka port UDP"
	}
	return "Aktif"
}

// defaultIface membaca /proc/net/route dan mengambil interface dengan tujuan 0.0.0.0.
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

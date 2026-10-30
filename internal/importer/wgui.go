package importer

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

type wguiClient struct {
	Name         string    `json:"name"`
	PrivateKey   string    `json:"private_key"`
	PublicKey    string    `json:"public_key"`
	PresharedKey string    `json:"preshared_key"`
	AllocatedIPs []string  `json:"allocated_ips"`
	AllowedIPs   []string  `json:"allowed_ips"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func ParseWGUI(files map[string][]byte) (Source, error) {
	out := Source{Clients: []Candidate{}}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		normalized := strings.ReplaceAll(n, "\\", "/")
		if !strings.HasSuffix(strings.ToLower(normalized), ".json") {
			continue
		}
		// Unggahan satuan tidak punya folder; isi arsip dikenali dari foldernya (db/clients, db/server).
		single := !strings.Contains(normalized, "/")
		folder := path.Base(path.Dir(normalized))
		if !single && folder != "clients" && folder != "server" {
			continue // mis. db/users, db/wake_on_lan_hosts
		}
		switch path.Base(normalized) {
		case "interfaces.json":
			var v struct {
				Addresses  []string    `json:"addresses"`
				ListenPort json.Number `json:"listen_port"`
			}
			if e := json.Unmarshal(files[n], &v); e != nil {
				return out, e
			}
			if len(v.Addresses) > 0 {
				out.Server.Address = v.Addresses[0]
			}
			out.Server.ListenPort, _ = strconv.Atoi(v.ListenPort.String())
			continue
		case "keypair.json":
			var v struct {
				PrivateKey string `json:"private_key"`
			}
			if e := json.Unmarshal(files[n], &v); e != nil {
				return out, e
			}
			out.Server.PrivateKey = v.PrivateKey
			continue
		case "global_settings.json":
			var v struct {
				DNSServers          []string    `json:"dns_servers"`
				MTU                 json.Number `json:"mtu"`
				PersistentKeepalive json.Number `json:"persistent_keepalive"`
			}
			if e := json.Unmarshal(files[n], &v); e != nil {
				return out, e
			}
			out.DefaultDNS = v.DNSServers
			out.DefaultMTU, _ = strconv.Atoi(v.MTU.String())
			out.DefaultKeepalive, _ = strconv.Atoi(v.PersistentKeepalive.String())
			continue
		}
		if folder == "server" {
			continue // metadata server lain, mis. hashes.json
		}
		var x wguiClient
		if err := json.Unmarshal(files[n], &x); err != nil {
			return out, fmt.Errorf("%s: %w", n, err)
		}
		if single && x.PublicKey == "" && x.PrivateKey == "" {
			continue // bukan berkas perangkat
		}
		if x.PublicKey == "" || x.PrivateKey == "" {
			return out, fmt.Errorf("%s: kunci perangkat tidak lengkap", n)
		}
		addr := ""
		for _, s := range x.AllocatedIPs {
			p, e := netip.ParsePrefix(s)
			if e == nil && p.Bits() == 32 && p.Addr().Is4() {
				addr = p.Addr().String()
				break
			}
		}
		if addr == "" {
			return out, fmt.Errorf("%s: tidak ada allocated_ips IPv4 /32", n)
		}
		out.Clients = append(out.Clients, Candidate{Name: x.Name, PrivateKey: x.PrivateKey, PublicKey: x.PublicKey, PresharedKey: x.PresharedKey, Address: addr, AllowedIPs: x.AllowedIPs, Enabled: x.Enabled, HasPrivateKey: true, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt})
	}
	if len(out.Clients) == 0 {
		return out, fmt.Errorf("tidak ada berkas perangkat di clients/")
	}
	return out, nil
}

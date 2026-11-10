package importer

import (
	"bufio"
	"bytes"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

func ParseWGConf(data []byte) (Source, error) {
	var out Source
	section, pending, name := "", "", ""
	fields := map[string]string{}
	flush := func() error {
		if section != "peer" {
			return nil
		}
		pub := strings.TrimSpace(fields["publickey"])
		if pub == "" {
			return fmt.Errorf("peer without PublicKey")
		}
		addr := ""
		for _, v := range strings.Split(fields["allowedips"], ",") {
			p, e := netip.ParsePrefix(strings.TrimSpace(v))
			if e == nil && p.Bits() == 32 && p.Addr().Is4() {
				addr = p.Addr().String()
				break
			}
		}
		if addr == "" {
			return fmt.Errorf("peer %s has no IPv4 /32 address", pub)
		}
		ka, _ := strconv.Atoi(fields["persistentkeepalive"])
		if name == "" {
			name = fmt.Sprintf("Import %d", len(out.Clients)+1)
		}
		out.Clients = append(out.Clients, Candidate{Name: name, PublicKey: pub, PresharedKey: fields["presharedkey"], Address: addr, Keepalive: ka, Enabled: true})
		return nil
	}
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 4096), 2<<20)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(line, "#") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			lower := strings.ToLower(v)
			switch {
			case strings.HasPrefix(lower, "name:"):
				v = strings.TrimSpace(v[5:])
			case strings.HasPrefix(lower, "client:"):
				v = strings.TrimSpace(v[7:])
			case strings.Contains(v, ":") || strings.HasPrefix(lower, "created") || strings.HasPrefix(lower, "updated"):
				continue
			}
			if v != "" {
				if section == "peer" && fields["publickey"] == "" {
					name = v
				} else {
					pending = v
				}
			}
			continue
		}
		if strings.HasPrefix(line, "[") {
			if err := flush(); err != nil {
				return out, err
			}
			switch strings.ToLower(line) {
			case "[interface]":
				section = "interface"
			case "[peer]":
				section = "peer"
				name = pending
			default:
				return out, fmt.Errorf("unknown section %s", line)
			}
			pending = ""
			fields = map[string]string{}
			continue
		}
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(strings.SplitN(v, "#", 2)[0])
		k = strings.ToLower(strings.TrimSpace(k))
		if section == "peer" && pending != "" && fields["publickey"] != "" {
			name = pending
			pending = ""
		}
		fields[k] = v
		if section == "interface" {
			switch k {
			case "privatekey":
				out.Server.PrivateKey = v
			case "address":
				out.Server.Address = strings.TrimSpace(strings.Split(v, ",")[0])
			case "listenport":
				out.Server.ListenPort, _ = strconv.Atoi(v)
			case "mtu":
				out.Server.MTU, _ = strconv.Atoi(v)
			}
		}
	}
	if err := scan.Err(); err != nil {
		return out, err
	}
	if section == "peer" && pending != "" {
		name = pending
	}
	if err := flush(); err != nil {
		return out, err
	}
	if out.Server.Address == "" || out.Server.PrivateKey == "" {
		return out, fmt.Errorf("Interface requires Address and PrivateKey")
	}
	return out, nil
}

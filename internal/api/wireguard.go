package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/luzyver/lzvr-umbra/internal/firewall"
	"github.com/luzyver/lzvr-umbra/internal/store"
	"github.com/luzyver/lzvr-umbra/internal/system"
	"github.com/luzyver/lzvr-umbra/internal/wg"
)

var ifaceRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,15}$`)

// ---- nilai bawaan ----

func defaultServer() store.ServerConfig {
	return store.ServerConfig{
		Interface:    "wg0",
		Address:      "10.8.0.1/24",
		ListenPort:   51820,
		EndpointHost: "vpn.luzyver.id",
		MTU:          1420,
	}
}

func (s *Server) loadServer(ctx context.Context) (store.ServerConfig, error) {
	srv := defaultServer()
	_, err := s.st.GetJSON(ctx, store.KeyServer, &srv)
	return srv, err
}

func (s *Server) loadDefaults(ctx context.Context, srv store.ServerConfig) (store.ClientDefaults, error) {
	d := store.ClientDefaults{
		DNS:        []string{"8.8.8.8", "9.9.9.9"},
		AllowedIPs: []string{"0.0.0.0/0"},
		Keepalive:  25,
		MTU:        1420,
	}
	if s.ag.Enabled() {
		if p, err := netip.ParsePrefix(srv.Address); err == nil {
			d.DNS = []string{p.Addr().String()}
		}
	}
	_, err := s.st.GetJSON(ctx, store.KeyDefaults, &d)
	return d, err
}

func (s *Server) loadFirewall(ctx context.Context) (store.FirewallState, error) {
	var f store.FirewallState
	_, err := s.st.GetJSON(ctx, store.KeyFirewall, &f)
	return f, err
}

func firewallRules(srv store.ServerConfig, mss bool) firewall.Rules {
	subnet := srv.Address
	if p, err := netip.ParsePrefix(srv.Address); err == nil {
		subnet = p.Masked().String()
	}
	return firewall.Rules{WGInterface: srv.Interface, OutInterface: srv.OutInterface, Subnet: subnet, MSSClamp: mss}
}

// syncLocked menyamakan kernel dengan database dan menulis cadangan conf.
// Pemanggil wajib memegang s.mu.
func (s *Server) syncLocked(ctx context.Context) error {
	srv, err := s.loadServer(ctx)
	if err != nil {
		return err
	}
	if !srv.Initialized {
		return nil
	}
	clients, err := s.st.ListClients(ctx)
	if err != nil {
		return err
	}
	if err := s.wg.Sync(srv, wg.PeersFromClients(clients)); err != nil {
		return fmt.Errorf("sinkron ke WireGuard: %w", err)
	}
	return s.writeBackup(srv, clients)
}

func (s *Server) writeBackup(srv store.ServerConfig, clients []store.Client) error {
	path := filepath.Join(s.cfg.DataDir, srv.Interface+".conf")
	if err := writeFileAtomic(path, []byte(wg.RenderServerConfig(srv, clients)), 0o600); err != nil {
		return fmt.Errorf("tulis cadangan conf: %w", err)
	}
	return nil
}

// Bootstrap dijalankan saat start: memasang lagi interface, peer, dan firewall
// dari database (kernel kosong setelah reboot). Kesalahan hanya dicatat.
func (s *Server) Bootstrap(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	srv, err := s.loadServer(ctx)
	if err != nil {
		slog.Error("bootstrap: baca server", "err", err)
		return
	}
	if !srv.Initialized {
		slog.Info("WireGuard belum diatur, buka panel untuk menjalankan wizard setup")
		return
	}
	if err := s.wg.EnsureInterface(srv); err != nil {
		slog.Error("bootstrap: interface", "err", err)
		return
	}
	if err := s.syncLocked(ctx); err != nil {
		slog.Error("bootstrap: sinkron", "err", err)
	}
	fw, err := s.loadFirewall(ctx)
	if err != nil {
		slog.Error("bootstrap: baca firewall", "err", err)
		return
	}
	if fw.Applied {
		if err := s.fw.ApplyNow(firewallRules(srv, fw.MSSClamp)); err != nil {
			slog.Error("bootstrap: firewall", "err", err)
		}
	}
	slog.Info("WireGuard aktif", "interface", srv.Interface)
}

// ---- cek sistem ----

func (s *Server) handleSystemCheck(w http.ResponseWriter, r *http.Request) {
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "baca server", err)
		return
	}
	port := srv.ListenPort
	if srv.Initialized {
		port = 0 // port sudah dipakai interface kita sendiri, tidak perlu dicek
	}
	writeJSON(w, http.StatusOK, system.Check(r.Context(), port))
}

// ---- server ----

func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "baca server", err)
		return
	}
	srv.PrivateKey = ""
	writeJSON(w, http.StatusOK, srv)
}

type serverInput struct {
	Interface    string `json:"interface"`
	Address      string `json:"address"`
	ListenPort   int    `json:"listen_port"`
	EndpointHost string `json:"endpoint_host"`
	EndpointPort int    `json:"endpoint_port"`
	MTU          int    `json:"mtu"`
	OutInterface string `json:"out_interface"`
}

func (in *serverInput) validate() error {
	in.EndpointHost = strings.TrimSpace(in.EndpointHost)
	if !ifaceRe.MatchString(in.Interface) {
		return fmt.Errorf("nama interface tidak valid (maks. 15 karakter: huruf, angka, titik, garis bawah, strip)")
	}
	p, err := netip.ParsePrefix(in.Address)
	if err != nil || !p.Addr().Is4() || p.Bits() < 16 || p.Bits() > 30 {
		return fmt.Errorf("alamat server harus IPv4 dengan prefix /16 sampai /30, mis. 10.8.0.1/24")
	}
	if p.Addr() == p.Masked().Addr() {
		return fmt.Errorf("alamat server tidak boleh alamat network (pakai mis. %s)", p.Masked().Addr().Next())
	}
	if in.ListenPort < 1 || in.ListenPort > 65535 {
		return fmt.Errorf("port harus 1 sampai 65535")
	}
	if in.EndpointPort < 0 || in.EndpointPort > 65535 {
		return fmt.Errorf("port publik harus 0 (sama dengan port) sampai 65535")
	}
	badHost := in.EndpointHost == "" || len(in.EndpointHost) > 253 ||
		strings.ContainsAny(in.EndpointHost, " \t\r\n/[]") ||
		(strings.Contains(in.EndpointHost, ":") && net.ParseIP(in.EndpointHost) == nil)
	if badHost {
		return fmt.Errorf("endpoint harus domain atau IP publik tanpa port, mis. vpn.contoh.id")
	}
	if in.MTU != 0 && (in.MTU < 1280 || in.MTU > 1500) {
		return fmt.Errorf("MTU harus 0 (otomatis) atau 1280 sampai 1500")
	}
	if !ifaceRe.MatchString(in.OutInterface) {
		return fmt.Errorf("interface internet tidak valid, mis. eth0")
	}
	return nil
}

func (s *Server) handlePutServer(w http.ResponseWriter, r *http.Request) {
	var in serverInput
	if !readJSON(w, r, &in) {
		return
	}
	if err := in.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	s.mu.Lock()
	defer s.mu.Unlock()

	old, err := s.loadServer(ctx)
	if err != nil {
		internalError(w, "baca server", err)
		return
	}
	clients, err := s.st.ListClients(ctx)
	if err != nil {
		internalError(w, "baca klien", err)
		return
	}
	if old.Initialized && in.Interface != old.Interface {
		writeError(w, http.StatusConflict, "nama interface tidak bisa diganti setelah setup")
		return
	}
	oldP, _ := netip.ParsePrefix(old.Address)
	newP, _ := netip.ParsePrefix(in.Address)
	if len(clients) > 0 && oldP.Masked() != newP.Masked() {
		writeError(w, http.StatusConflict, "subnet tidak bisa diganti selama masih ada klien")
		return
	}
	for _, c := range clients {
		if c.Address == newP.Addr().String() {
			writeError(w, http.StatusConflict, "alamat server bentrok dengan klien "+c.Name)
			return
		}
	}

	srv := old
	srv.Interface = in.Interface
	srv.Address = in.Address
	srv.ListenPort = in.ListenPort
	srv.EndpointHost = in.EndpointHost
	srv.EndpointPort = in.EndpointPort
	srv.MTU = in.MTU
	srv.OutInterface = in.OutInterface
	if srv.PrivateKey == "" {
		kp, err := wg.NewKeyPair()
		if err != nil {
			internalError(w, "buat kunci server", err)
			return
		}
		srv.PrivateKey, srv.PublicKey = kp.Private, kp.Public
	}
	if err := s.wg.EnsureInterface(srv); err != nil {
		writeError(w, http.StatusBadGateway, "gagal menyiapkan interface: "+err.Error())
		return
	}
	if err := s.wg.Sync(srv, wg.PeersFromClients(clients)); err != nil {
		writeError(w, http.StatusBadGateway, "gagal menerapkan ke WireGuard: "+err.Error())
		return
	}
	srv.Initialized = true
	if err := s.st.SetJSON(ctx, store.KeyServer, srv); err != nil {
		internalError(w, "simpan server", err)
		return
	}
	warnings := []string{}
	if err := s.writeBackup(srv, clients); err != nil {
		warnings = append(warnings, err.Error())
	}
	if fw, err := s.loadFirewall(ctx); err == nil && fw.Applied {
		if err := s.fw.ApplyNow(firewallRules(srv, fw.MSSClamp)); err != nil {
			warnings = append(warnings, "firewall: "+err.Error())
		}
	}
	// Profil klien memuat endpoint dan MTU server; bila berubah, klien harus unduh ulang.
	profileChanged := old.Initialized && len(clients) > 0 &&
		(old.EndpointHost != srv.EndpointHost || endpointOf(old) != endpointOf(srv))
	srv.PrivateKey = ""
	writeJSON(w, http.StatusOK, map[string]any{"server": srv, "profile_changed": profileChanged, "warnings": warnings})
}

func endpointOf(srv store.ServerConfig) int {
	if srv.EndpointPort > 0 {
		return srv.EndpointPort
	}
	return srv.ListenPort
}

// ---- bawaan klien ----

func (s *Server) handleGetDefaults(w http.ResponseWriter, r *http.Request) {
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "baca server", err)
		return
	}
	d, err := s.loadDefaults(r.Context(), srv)
	if err != nil {
		internalError(w, "baca bawaan klien", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handlePutDefaults(w http.ResponseWriter, r *http.Request) {
	var d store.ClientDefaults
	if !readJSON(w, r, &d) {
		return
	}
	var err error
	if d.DNS, err = cleanIPs(d.DNS); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if d.AllowedIPs, err = cleanCIDRs(d.AllowedIPs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := checkKeepaliveMTU(d.Keepalive, d.MTU); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.st.SetJSON(r.Context(), store.KeyDefaults, d); err != nil {
		internalError(w, "simpan bawaan klien", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func cleanIPs(in []string) ([]string, error) {
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("DNS %q bukan alamat IP", v)
		}
		out = append(out, ip.String())
	}
	if len(out) > 4 {
		return nil, fmt.Errorf("DNS maksimal 4 alamat")
	}
	return out, nil
}

func cleanCIDRs(in []string) ([]string, error) {
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		p, err := netip.ParsePrefix(v)
		if err != nil {
			return nil, fmt.Errorf("AllowedIPs %q bukan CIDR, mis. 0.0.0.0/0", v)
		}
		out = append(out, p.Masked().String())
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("AllowedIPs tidak boleh kosong")
	}
	return out, nil
}

func checkKeepaliveMTU(keepalive, mtu int) error {
	if keepalive < 0 || keepalive > 3600 {
		return fmt.Errorf("keepalive harus 0 sampai 3600 detik")
	}
	if mtu != 0 && (mtu < 1280 || mtu > 1500) {
		return fmt.Errorf("MTU harus 0 (otomatis) atau 1280 sampai 1500")
	}
	return nil
}

// ---- firewall ----

func (s *Server) handleGetFirewall(w http.ResponseWriter, r *http.Request) {
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "baca server", err)
		return
	}
	state, err := s.loadFirewall(r.Context())
	if err != nil {
		internalError(w, "baca firewall", err)
		return
	}
	rules := firewallRules(srv, state.MSSClamp)
	preview, perr := firewall.Render(rules)
	resp := map[string]any{"state": state, "pending": s.fw.Pending(), "rules": rules, "preview": preview}
	if perr != nil {
		resp["preview_error"] = perr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleApplyFirewall(w http.ResponseWriter, r *http.Request) {
	var in struct {
		MSSClamp bool `json:"mss_clamp"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "baca server", err)
		return
	}
	if !srv.Initialized {
		writeError(w, http.StatusConflict, "atur WireGuard dulu sebelum firewall")
		return
	}
	state, err := s.loadFirewall(r.Context())
	if err != nil {
		internalError(w, "baca firewall", err)
		return
	}
	var prev *firewall.Rules
	if state.Applied {
		p := firewallRules(srv, state.MSSClamp)
		prev = &p
	}
	if err := s.fw.ApplyTrial(firewallRules(srv, in.MSSClamp), firewallTrial, prev); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pending": s.fw.Pending()})
}

func (s *Server) handleConfirmFirewall(w http.ResponseWriter, r *http.Request) {
	rules, err := s.fw.Confirm()
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	state := store.FirewallState{Applied: true, MSSClamp: rules.MSSClamp}
	if err := s.st.SetJSON(r.Context(), store.KeyFirewall, state); err != nil {
		internalError(w, "simpan firewall", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": state})
}

func (s *Server) handleRemoveFirewall(w http.ResponseWriter, r *http.Request) {
	if err := s.fw.Remove(); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	state, _ := s.loadFirewall(r.Context())
	state.Applied = false
	if err := s.st.SetJSON(r.Context(), store.KeyFirewall, state); err != nil {
		internalError(w, "simpan firewall", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": state})
}

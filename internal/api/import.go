package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"

	"github.com/luzyver/lzvr-umbra/internal/importer"
	"github.com/luzyver/lzvr-umbra/internal/store"
	"github.com/luzyver/lzvr-umbra/internal/wg"
)

type importPreview struct {
	Rows             []importer.Candidate `json:"rows"`
	New              int                  `json:"new"`
	Skipped          int                  `json:"skipped"`
	ServerAddress    string               `json:"server_address"`
	ServerListenPort int                  `json:"server_listen_port"`
	ServerKeyChanges bool                 `json:"server_key_changes"`
	AdoptServer      bool                 `json:"adopt_server"`
	Warnings         []string             `json:"warnings"`
	Signature        string               `json:"signature"`
}

func importUpload(w http.ResponseWriter, r *http.Request) (importer.Source, []byte, bool, error) {
	r.Body = http.MaxBytesReader(w, r.Body, importer.MaxUpload+65536)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return importer.Source{}, nil, false, fmt.Errorf("unggahan terlalu besar atau tidak valid: %w", err)
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	source := r.FormValue("source")
	adopt := r.FormValue("adopt_server") == "true"
	files := map[string][]byte{}
	var raw []byte
	for _, hs := range r.MultipartForm.File["files"] {
		f, e := hs.Open()
		if e != nil {
			return importer.Source{}, nil, false, e
		}
		b, e := io.ReadAll(io.LimitReader(f, importer.MaxUpload+1))
		f.Close()
		if e != nil {
			return importer.Source{}, nil, false, e
		}
		if len(b) > importer.MaxUpload {
			return importer.Source{}, nil, false, fmt.Errorf("berkas terlalu besar")
		}
		raw = append(raw, []byte(hs.Filename)...)
		raw = append(raw, b...)
		if source == "wgconf" {
			files["wg0.conf"] = b
			continue
		}
		lower := strings.ToLower(hs.Filename)
		if strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
			part, e := importer.Extract(hs.Filename, b)
			if e != nil {
				return importer.Source{}, nil, false, e
			}
			for n, v := range part {
				files[n] = v
			}
		} else {
			files[hs.Filename] = b
		}
	}
	if source == "wgconf" {
		b := []byte(r.FormValue("text"))
		if len(b) == 0 {
			b = files["wg0.conf"]
		}
		raw = append(raw, b...)
		x, e := importer.ParseWGConf(b)
		return x, raw, adopt, e
	}
	if source == "wgui" {
		x, e := importer.ParseWGUI(files)
		return x, raw, adopt, e
	}
	return importer.Source{}, nil, false, fmt.Errorf("sumber impor tidak dikenal")
}

func (s *Server) prepareImport(w http.ResponseWriter, r *http.Request) (importer.Source, importPreview, bool) {
	src, raw, adopt, err := importUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return src, importPreview{}, false
	}
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "baca server", err)
		return src, importPreview{}, false
	}
	if r.FormValue("adopt_server") == "" {
		adopt = !srv.Initialized
	}
	if src.Server.PrivateKey != "" && adopt && (src.Server.ListenPort < 1 || src.Server.ListenPort > 65535) {
		writeError(w, http.StatusBadRequest, "port server sumber harus 1 sampai 65535")
		return src, importPreview{}, false
	}
	if !srv.Initialized && src.Server.Address != "" {
		p, e := netip.ParsePrefix(src.Server.Address)
		if e != nil || !p.Addr().Is4() || p.Bits() < 16 || p.Bits() > 30 || p.Addr() == p.Masked().Addr() {
			writeError(w, http.StatusBadRequest, "alamat server sumber harus IPv4 /16 sampai /30")
			return src, importPreview{}, false
		}
		srv.Address = src.Server.Address
	}
	if _, err = netip.ParsePrefix(srv.Address); err != nil {
		writeError(w, http.StatusBadRequest, "subnet server tidak valid")
		return src, importPreview{}, false
	}
	defaults, err := s.loadDefaults(r.Context(), srv)
	if err != nil {
		internalError(w, "baca bawaan", err)
		return src, importPreview{}, false
	}
	existing, err := s.st.ListClients(r.Context())
	if err != nil {
		internalError(w, "baca perangkat", err)
		return src, importPreview{}, false
	}
	old := make([]importer.Candidate, 0, len(existing))
	for _, c := range existing {
		old = append(old, importer.Candidate{PublicKey: c.PublicKey, Address: c.Address})
	}
	for i := range src.Clients {
		c := &src.Clients[i]
		if c.Name == "" {
			c.Name = fmt.Sprintf("Impor %d", i+1)
		}
		if c.Name, err = cleanName(c.Name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return src, importPreview{}, false
		}
		if !wg.ValidKey(c.PublicKey) || (c.PrivateKey != "" && !wg.ValidKey(c.PrivateKey)) || (c.PresharedKey != "" && !wg.ValidKey(c.PresharedKey)) {
			c.Reason = "kunci WireGuard tidak valid"
			continue
		}
		if c.PrivateKey != "" {
			if pub, e := wg.PublicKeyOf(c.PrivateKey); e != nil || pub != c.PublicKey {
				c.Reason = "pasangan kunci tidak cocok"
				continue
			}
		}
		if len(c.AllowedIPs) == 0 {
			c.AllowedIPs = defaults.AllowedIPs
		}
		if len(c.DNS) == 0 {
			c.DNS = src.DefaultDNS
		}
		if len(c.DNS) == 0 {
			c.DNS = defaults.DNS
		}
		if c.MTU == 0 {
			c.MTU = src.DefaultMTU
		}
		if c.MTU == 0 {
			c.MTU = defaults.MTU
		}
		if c.Keepalive == 0 {
			c.Keepalive = src.DefaultKeepalive
		}
		if c.AllowedIPs, err = cleanCIDRs(c.AllowedIPs); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return src, importPreview{}, false
		}
		if c.DNS, err = cleanIPs(c.DNS); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return src, importPreview{}, false
		}
		if err = checkKeepaliveMTU(c.Keepalive, c.MTU); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return src, importPreview{}, false
		}
	}
	rows := importer.Check(src.Clients, old, srv.Address)
	p := importPreview{Rows: rows, ServerAddress: srv.Address, ServerListenPort: src.Server.ListenPort, AdoptServer: adopt, Warnings: []string{}}
	for _, c := range rows {
		if c.Reason == "" {
			p.New++
		} else {
			p.Skipped++
		}
	}
	if src.Server.PrivateKey != "" {
		pub, e := wg.PublicKeyOf(src.Server.PrivateKey)
		if e != nil {
			writeError(w, http.StatusBadRequest, "kunci server tidak valid")
			return src, p, false
		}
		p.ServerKeyChanges = srv.PrivateKey != "" && pub != srv.PublicKey
		if p.ServerKeyChanges && adopt {
			p.Warnings = append(p.Warnings, "Kunci server akan berubah; perangkat lama yang memakai kunci server Umbra harus memperbarui profil.")
		}
		if p.ServerKeyChanges && !adopt {
			p.Warnings = append(p.Warnings, "Kunci server berbeda; perangkat yang diimpor perlu profil server Umbra agar tersambung.")
		}
	}
	if !srv.Initialized {
		p.Warnings = append(p.Warnings, "Lengkapi pengaturan server sebelum perangkat bisa tersambung.")
		if src.Server.PrivateKey == "" {
			p.Warnings = append(p.Warnings, "Kunci server tidak ada dalam unggahan; impor keypair.json agar profil lama tetap bisa tersambung.")
		}
	}
	digest := sha256.New()
	digest.Write(raw)
	state, _ := json.Marshal(struct {
		PublicKey   string
		Address     string
		ListenPort  int
		Initialized bool
	}{srv.PublicKey, srv.Address, srv.ListenPort, srv.Initialized})
	digest.Write(state)
	b, _ := json.Marshal(p)
	digest.Write(b)
	p.Signature = hex.EncodeToString(digest.Sum(nil))
	src.Clients = rows
	return src, p, true
}

func (s *Server) handleImportPreview(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, p, ok := s.prepareImport(w, r)
	if ok {
		writeJSON(w, http.StatusOK, p)
	}
}

func (s *Server) handleImportCommit(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	expected := r.URL.Query().Get("signature")
	src, p, ok := s.prepareImport(w, r)
	if !ok {
		return
	}
	if expected == "" || expected != p.Signature {
		writeError(w, http.StatusConflict, "pratinjau berubah; periksa ulang sebelum menyimpan")
		return
	}
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "baca server", err)
		return
	}
	var update *store.ServerConfig
	if !srv.Initialized && src.Server.Address != "" {
		srv.Address = src.Server.Address
		update = &srv
	}
	if p.AdoptServer && src.Server.PrivateKey != "" {
		srv.PrivateKey = src.Server.PrivateKey
		srv.PublicKey, _ = wg.PublicKeyOf(srv.PrivateKey)
		if src.Server.ListenPort > 0 {
			srv.ListenPort = src.Server.ListenPort
			if srv.EndpointPort == 0 {
				srv.EndpointPort = src.Server.ListenPort
			}
		}
		update = &srv
	}
	if update != nil && srv.Initialized {
		if err = s.wg.EnsureInterface(srv); err != nil {
			writeError(w, http.StatusBadGateway, "gagal menyiapkan interface: "+err.Error())
			return
		}
	}
	clients := []store.Client{}
	for _, c := range src.Clients {
		if c.Reason == "" {
			clients = append(clients, c.Client())
		}
	}
	saved, err := s.st.ImportClients(r.Context(), clients, update)
	if err != nil {
		internalError(w, "simpan perangkat", err)
		return
	}
	if err = s.syncLocked(r.Context()); err != nil {
		internalError(w, "perangkat tersimpan tetapi gagal diterapkan", err)
		return
	}
	warnings := p.Warnings
	for _, c := range saved {
		s.agSync(r.Context(), c, &warnings)
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": len(saved), "skipped": p.Skipped, "warnings": warnings})
}

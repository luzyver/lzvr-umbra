package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/luzyver/lzvr-umbra/internal/store"
	"github.com/luzyver/lzvr-umbra/internal/wg"
)

var unsafeFileRe = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := len([]rune(name))
	if n < 1 || n > 64 {
		return "", errors.New("client name must be 1 to 64 characters")
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 || strings.ContainsAny(name, "[]") {
		return "", errors.New("client name must not contain control characters or [ ]")
	}
	return name, nil
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid client id")
		return 0, false
	}
	return id, true
}

func (s *Server) clientOr404(w http.ResponseWriter, r *http.Request) (store.Client, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return store.Client{}, false
	}
	c, err := s.st.GetClient(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return c, false
	}
	if err != nil {
		internalError(w, "read client", err)
		return c, false
	}
	return c, true
}

// nextAddress finds the first free IP in the server subnet.
func nextAddress(srv store.ServerConfig, clients []store.Client) (string, error) {
	p, err := netip.ParsePrefix(srv.Address)
	if err != nil {
		return "", fmt.Errorf("invalid server address: %w", err)
	}
	used := map[netip.Addr]bool{p.Addr(): true}
	for _, c := range clients {
		if a, err := netip.ParseAddr(c.Address); err == nil {
			used[a] = true
		}
	}
	net := p.Masked()
	for a := net.Addr().Next(); net.Contains(a); a = a.Next() {
		if !net.Contains(a.Next()) {
			break // broadcast address
		}
		if !used[a] {
			return a.String(), nil
		}
	}
	return "", errors.New("subnet is full, no free IP for a new client")
}

// agSync registers the client with AdGuard. A failure does not roll back the
// change; it is only returned as a warning.
func (s *Server) agSync(ctx context.Context, c store.Client, warnings *[]string) {
	if !s.ag.Enabled() {
		return
	}
	if err := s.ag.Upsert(ctx, c.ID, c.Name, c.Address, c.Adblock); err != nil {
		internalLog("adguard upsert", err)
		*warnings = append(*warnings, "AdGuard: "+err.Error())
	}
}

func (s *Server) handleListClients(w http.ResponseWriter, r *http.Request) {
	clients, err := s.st.ListClients(r.Context())
	if err != nil {
		internalError(w, "read client", err)
		return
	}
	if clients == nil {
		clients = []store.Client{}
	}
	writeJSON(w, http.StatusOK, clients)
}

func (s *Server) handleGetClient(w http.ResponseWriter, r *http.Request) {
	if c, ok := s.clientOr404(w, r); ok {
		writeJSON(w, http.StatusOK, c)
	}
}

type clientInput struct {
	Name       *string  `json:"name"`
	AllowedIPs []string `json:"allowed_ips"`
	DNS        []string `json:"dns"`
	MTU        *int     `json:"mtu"`
	Keepalive  *int     `json:"keepalive"`
	Enabled    *bool    `json:"enabled"`
	Adblock    *bool    `json:"adblock"`
}

// apply fills c from the input; only fields that were sent are changed.
// It returns true if the client profile content changed (needs a re-download).
func (in clientInput) apply(c *store.Client) (bool, error) {
	changed := false
	if in.Name != nil {
		n, err := cleanName(*in.Name)
		if err != nil {
			return false, err
		}
		c.Name = n
	}
	if in.AllowedIPs != nil {
		v, err := cleanCIDRs(in.AllowedIPs)
		if err != nil {
			return false, err
		}
		changed = changed || !slices.Equal(v, c.AllowedIPs)
		c.AllowedIPs = v
	}
	if in.DNS != nil {
		v, err := cleanIPs(in.DNS)
		if err != nil {
			return false, err
		}
		changed = changed || !slices.Equal(v, c.DNS)
		c.DNS = v
	}
	if in.MTU != nil {
		changed = changed || *in.MTU != c.MTU
		c.MTU = *in.MTU
	}
	if in.Keepalive != nil {
		changed = changed || *in.Keepalive != c.Keepalive
		c.Keepalive = *in.Keepalive
	}
	if err := checkKeepaliveMTU(c.Keepalive, c.MTU); err != nil {
		return false, err
	}
	if in.Enabled != nil {
		c.Enabled = *in.Enabled
	}
	if in.Adblock != nil {
		c.Adblock = *in.Adblock
	}
	return changed, nil
}

func (s *Server) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	var in clientInput
	if !readJSON(w, r, &in) {
		return
	}
	if in.Name == nil {
		writeError(w, http.StatusBadRequest, "client name is required")
		return
	}
	ctx := r.Context()
	s.mu.Lock()
	defer s.mu.Unlock()

	srv, err := s.loadServer(ctx)
	if err != nil {
		internalError(w, "read server", err)
		return
	}
	if !srv.Initialized {
		writeError(w, http.StatusConflict, "set up WireGuard first via the setup wizard")
		return
	}
	d, err := s.loadDefaults(ctx, srv)
	if err != nil {
		internalError(w, "read client defaults", err)
		return
	}
	c := store.Client{
		AllowedIPs: d.AllowedIPs,
		DNS:        d.DNS,
		MTU:        d.MTU,
		Keepalive:  d.Keepalive,
		Enabled:    true,
		Adblock:    true,
	}
	if _, err := in.apply(&c); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	clients, err := s.st.ListClients(ctx)
	if err != nil {
		internalError(w, "read client", err)
		return
	}
	if c.Address, err = nextAddress(srv, clients); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	kp, err := wg.NewKeyPair()
	if err != nil {
		internalError(w, "generate client key", err)
		return
	}
	c.PrivateKey, c.PublicKey = kp.Private, kp.Public
	c.HasPrivateKey = true
	if c.PresharedKey, err = wg.NewPresharedKey(); err != nil {
		internalError(w, "generate preshared key", err)
		return
	}
	if err := s.st.CreateClient(ctx, &c); err != nil {
		internalError(w, "save client", err)
		return
	}
	if err := s.syncLocked(ctx); err != nil {
		// The data is already saved; sync will be retried on the next change or on restart.
		internalError(w, "client saved but failed to apply", err)
		return
	}
	warnings := []string{}
	s.agSync(ctx, c, &warnings)
	writeJSON(w, http.StatusCreated, map[string]any{"client": c, "warnings": warnings})
}

func (s *Server) handleUpdateClient(w http.ResponseWriter, r *http.Request) {
	var in clientInput
	if !readJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.clientOr404(w, r)
	if !ok {
		return
	}
	changed, err := in.apply(&c)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.st.UpdateClient(ctx, &c); err != nil {
		internalError(w, "save client", err)
		return
	}
	if err := s.syncLocked(ctx); err != nil {
		internalError(w, "client saved but failed to apply", err)
		return
	}
	warnings := []string{}
	s.agSync(ctx, c, &warnings)
	writeJSON(w, http.StatusOK, map[string]any{"client": c, "profile_changed": changed, "warnings": warnings})
}

func (s *Server) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.st.DeleteClient(ctx, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	} else if err != nil {
		internalError(w, "delete client", err)
		return
	}
	if err := s.syncLocked(ctx); err != nil {
		internalError(w, "client deleted but failed to apply", err)
		return
	}
	warnings := []string{}
	if s.ag.Enabled() {
		if err := s.ag.Delete(ctx, id); err != nil {
			internalLog("adguard delete", err)
			warnings = append(warnings, "AdGuard: "+err.Error())
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"warnings": warnings})
}

func (s *Server) clientProfile(w http.ResponseWriter, r *http.Request) (store.Client, string, bool) {
	c, ok := s.clientOr404(w, r)
	if !ok {
		return c, "", false
	}
	if c.PrivateKey == "" {
		writeError(w, http.StatusConflict, "private key is not available; regenerate the key (the device must scan the new profile)")
		return c, "", false
	}
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "read server", err)
		return c, "", false
	}
	return c, wg.RenderClientConfig(srv, c), true
}

func (s *Server) handleClientConfig(w http.ResponseWriter, r *http.Request) {
	c, conf, ok := s.clientProfile(w, r)
	if !ok {
		return
	}
	// The WireGuard app uses the file name as the tunnel name (max. 15 characters).
	name := strings.Trim(unsafeFileRe.ReplaceAllString(c.Name, "-"), "-.")
	if name == "" {
		name = "wg" + strconv.FormatInt(c.ID, 10)
	}
	if len(name) > 15 {
		name = name[:15]
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.conf"`)
	w.Write([]byte(conf))
}

func (s *Server) handleClientQR(w http.ResponseWriter, r *http.Request) {
	_, conf, ok := s.clientProfile(w, r)
	if !ok {
		return
	}
	png, err := qrcode.Encode(conf, qrcode.Medium, 512)
	if err != nil {
		internalError(w, "generate QR", err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}

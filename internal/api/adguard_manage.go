package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"time"

	"github.com/luzyver/lzvr-umbra/internal/adguard"
	"github.com/luzyver/lzvr-umbra/internal/auth"
	"github.com/luzyver/lzvr-umbra/internal/helper"
	"github.com/luzyver/lzvr-umbra/internal/store"
)

// agJob is the state of a background AdGuard install/uninstall job.
type agJob struct {
	Action  string `json:"action"`
	State   string `json:"state"` // idle | running | done | error
	Message string `json:"message"`
}

var defaultDNS = []string{"8.8.8.8", "9.9.9.9"}

// loadAdGuardConn uses the AdGuard connection stored in the database if there is one;
// otherwise it keeps the values from the environment.
func (s *Server) loadAdGuardConn(ctx context.Context) {
	var conn store.AdGuardConn
	found, err := s.st.GetJSON(ctx, store.KeyAdGuard, &conn)
	if err != nil {
		internalLog("read AdGuard connection", err)
		return
	}
	if found && conn.URL != "" {
		s.ag.Configure(conn.URL, conn.User, conn.Pass)
	}
}

// currentConn returns the active connection and where it came from: db, env, or none.
func (s *Server) currentConn(ctx context.Context) (store.AdGuardConn, string, error) {
	var conn store.AdGuardConn
	found, err := s.st.GetJSON(ctx, store.KeyAdGuard, &conn)
	if err != nil {
		return conn, "", err
	}
	if found && conn.URL != "" {
		return conn, "db", nil
	}
	if s.cfg.AdGuardURL != "" {
		return store.AdGuardConn{URL: s.cfg.AdGuardURL, User: s.cfg.AdGuardUser, Pass: s.cfg.AdGuardPass}, "env", nil
	}
	return store.AdGuardConn{}, "none", nil
}

func (s *Server) tunnelIP(srv store.ServerConfig) (string, error) {
	p, err := netip.ParsePrefix(srv.Address)
	if err != nil {
		return "", fmt.Errorf("invalid server address %q", srv.Address)
	}
	return p.Addr().String(), nil
}

// ---------- background jobs ----------

func (s *Server) startAGJob(action string, fn func(ctx context.Context) (string, error)) bool {
	s.agJobMu.Lock()
	if s.agJob.State == "running" {
		s.agJobMu.Unlock()
		return false
	}
	s.agJob = agJob{Action: action, State: "running", Message: "starting..."}
	s.agJobMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		msg, err := fn(ctx)
		s.agJobMu.Lock()
		defer s.agJobMu.Unlock()
		if err != nil {
			slog.Error("AdGuard "+action+" failed", "err", err)
			s.agJob = agJob{Action: action, State: "error", Message: err.Error()}
			return
		}
		slog.Info("AdGuard "+action+" finished", "msg", msg)
		s.agJob = agJob{Action: action, State: "done", Message: msg}
	}()
	return true
}

func (s *Server) setAGJobMsg(msg string) {
	s.agJobMu.Lock()
	s.agJob.Message = msg
	s.agJobMu.Unlock()
}

// maybeAutoInstallAdGuard installs AdGuard automatically if the installer was run
// with --with-adguard (UMBRA_ADGUARD_AUTOINSTALL=true) and AdGuard is not yet connected.
func (s *Server) maybeAutoInstallAdGuard() {
	if !s.cfg.AdGuardAutoInstall || s.ag.Enabled() {
		return
	}
	s.startAGJob("install", s.installAdGuard)
}

// runHelper asks umbra-helper (root) to carry out req, then waits for the result.
// wait (optional) is called repeatedly after the helper finishes until it returns true.
func (s *Server) runHelper(ctx context.Context, req helper.Request, wait func() bool) (string, error) {
	dir := helper.Dir(s.cfg.DataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	_ = os.Remove(helper.StatusPath(s.cfg.DataDir))
	b, _ := json.Marshal(req)
	if err := writeFileAtomic(helper.RequestPath(s.cfg.DataDir), b, 0o600); err != nil {
		return "", err
	}

	start := time.Now()
	deadline := start.Add(6 * time.Minute)
	var msg string
	for {
		if err := sleepCtx(ctx, 2*time.Second); err != nil {
			return "", err
		}
		st, ok := helper.ReadStatus(s.cfg.DataDir)
		if !ok {
			if time.Since(start) > 30*time.Second {
				_ = os.Remove(helper.RequestPath(s.cfg.DataDir))
				return "", errors.New("helper is not responding (check: systemctl status umbra-helper.path)")
			}
			continue
		}
		if st.State == "error" {
			return "", errors.New(st.Message)
		}
		if st.State == "done" {
			msg = st.Message
			break
		}
		if time.Now().After(deadline) {
			return "", errors.New("helper is taking too long, check: journalctl -u umbra-helper")
		}
	}
	if wait == nil {
		return msg, nil
	}
	for range 30 {
		if wait() {
			return msg, nil
		}
		if err := sleepCtx(ctx, 2*time.Second); err != nil {
			return "", err
		}
	}
	return "", errors.New(msg + ", but AdGuard is not responding yet (check: systemctl status AdGuardHome)")
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (s *Server) installAdGuard(ctx context.Context) (string, error) {
	srv, err := s.loadServer(ctx)
	if err != nil {
		return "", err
	}
	if !srv.Initialized {
		return "", errors.New("set up the WireGuard server before installing AdGuard")
	}
	ip, err := s.tunnelIP(srv)
	if err != nil {
		return "", err
	}

	switch adguard.WizardState(ctx, adguard.WizardURL) {
	case "ready":
		return "", errors.New("AdGuard Home is already installed and set up at " + adguard.WizardURL +
			"; connect it with the existing login in the Connection tab")
	case "down":
		if !helper.Available() {
			return "", errors.New("the installer helper is missing on the server; re-run the latest install.sh, " +
				"or install AdGuard Home yourself and connect it in the Connection tab")
		}
		s.setAGJobMsg("downloading and installing AdGuard Home...")
		_, err := s.runHelper(ctx, helper.Request{Action: "install", WGInterface: srv.Interface}, func() bool {
			return adguard.WizardState(ctx, adguard.WizardURL) == "wizard"
		})
		if err != nil {
			return "", err
		}
	}

	s.setAGJobMsg("running AdGuard initial setup...")
	pass := auth.NewToken()
	if err := adguard.RunWizard(ctx, adguard.WizardURL, ip, "admin", pass); err != nil {
		return "", fmt.Errorf("AdGuard initial setup: %w", err)
	}
	// After setup, AdGuard restarts its web UI and DNS; wait until it is ready.
	var testErr error
	for range 15 {
		if err := sleepCtx(ctx, 2*time.Second); err != nil {
			return "", err
		}
		if _, testErr = s.ag.Test(ctx, adguard.WizardURL, "admin", pass); testErr == nil {
			break
		}
	}
	if testErr != nil {
		return "", fmt.Errorf("AdGuard is not ready after setup: %w", testErr)
	}

	conn := store.AdGuardConn{URL: adguard.WizardURL, User: "admin", Pass: pass, Managed: true}
	if err := s.st.SetJSON(ctx, store.KeyAdGuard, conn); err != nil {
		return "", err
	}
	s.ag.Configure(conn.URL, conn.User, conn.Pass)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.setDefaultDNS(ctx, srv, []string{ip}); err != nil {
		internalLog("update default DNS", err)
	}
	synced, _, warnings, err := s.agSyncAllLocked(ctx)
	if err != nil {
		return "", fmt.Errorf("AdGuard is installed, but device sync failed: %w", err)
	}
	msg := fmt.Sprintf("AdGuard Home installed and %d devices registered.", synced)
	if n := s.clientsWithoutDNS(ctx, ip); n > 0 {
		msg += fmt.Sprintf(" %d existing devices still use other DNS; re-download their profiles so ads get blocked too.", n)
	}
	for _, w := range warnings {
		msg += " Warning: " + w
	}
	return msg, nil
}

// setDefaultDNS replaces the default DNS for new clients. The caller holds s.mu.
func (s *Server) setDefaultDNS(ctx context.Context, srv store.ServerConfig, dns []string) error {
	d, err := s.loadDefaults(ctx, srv)
	if err != nil {
		return err
	}
	d.DNS = dns
	return s.st.SetJSON(ctx, store.KeyDefaults, d)
}

func (s *Server) clientsWithoutDNS(ctx context.Context, ip string) int {
	clients, err := s.st.ListClients(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, c := range clients {
		if !slices.Contains(c.DNS, ip) {
			n++
		}
	}
	return n
}

func (s *Server) uninstallAdGuard(ctx context.Context) (string, error) {
	conn, source, err := s.currentConn(ctx)
	if err != nil {
		return "", err
	}
	if source != "db" || !conn.Managed {
		return "", errors.New("this AdGuard was not installed by the panel; remove it on the server yourself, then disconnect it")
	}
	if !helper.Available() {
		return "", errors.New("the installer helper is missing on the server; re-run the latest install.sh")
	}
	srv, err := s.loadServer(ctx)
	if err != nil {
		return "", err
	}
	s.setAGJobMsg("uninstalling AdGuard Home...")
	msg, err := s.runHelper(ctx, helper.Request{Action: "uninstall", WGInterface: srv.Interface}, nil)
	if err != nil {
		return "", err
	}
	if err := s.disconnectAdGuard(ctx, srv); err != nil {
		return "", err
	}
	return msg + ". Devices using AdGuard DNS need to re-download their profiles.", nil
}

// disconnectAdGuard removes the connection from the database (falling back to the environment values)
// and restores the default DNS if it still points at AdGuard.
func (s *Server) disconnectAdGuard(ctx context.Context, srv store.ServerConfig) error {
	if err := s.st.SetJSON(ctx, store.KeyAdGuard, store.AdGuardConn{}); err != nil {
		return err
	}
	s.ag.Configure(s.cfg.AdGuardURL, s.cfg.AdGuardUser, s.cfg.AdGuardPass)
	if s.ag.Enabled() {
		return nil
	}
	ip, err := s.tunnelIP(srv)
	if err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.loadDefaults(ctx, srv)
	if err != nil {
		return err
	}
	if slices.Equal(d.DNS, []string{ip}) {
		d.DNS = defaultDNS
		return s.st.SetJSON(ctx, store.KeyDefaults, d)
	}
	return nil
}

// ---------- connection & install endpoints ----------

func (s *Server) handleAdGuardGetConn(w http.ResponseWriter, r *http.Request) {
	conn, source, err := s.currentConn(r.Context())
	if err != nil {
		internalError(w, "read AdGuard connection", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url": conn.URL, "user": conn.User, "has_password": conn.Pass != "",
		"managed": conn.Managed, "source": source,
	})
}

type agConnInput struct {
	URL  string `json:"url"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

// resolveConnInput validates the URL and fills in the existing password if empty.
func (s *Server) resolveConnInput(ctx context.Context, in agConnInput) (store.AdGuardConn, error) {
	u, err := url.Parse(in.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return store.AdGuardConn{}, errors.New("the AdGuard address must be a full http:// or https:// URL including the host")
	}
	conn := store.AdGuardConn{URL: in.URL, User: in.User, Pass: in.Pass}
	cur, _, err := s.currentConn(ctx)
	if err != nil {
		return conn, err
	}
	if conn.Pass == "" {
		conn.Pass = cur.Pass
	}
	conn.Managed = cur.Managed && cur.URL == conn.URL
	return conn, nil
}

func (s *Server) handleAdGuardTestConn(w http.ResponseWriter, r *http.Request) {
	var in agConnInput
	if !readJSON(w, r, &in) {
		return
	}
	conn, err := s.resolveConnInput(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	st, err := s.ag.Test(r.Context(), conn.URL, conn.User, conn.Pass)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": st})
}

func (s *Server) handleAdGuardPutConn(w http.ResponseWriter, r *http.Request) {
	var in agConnInput
	if !readJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	conn, err := s.resolveConnInput(ctx, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.ag.Test(ctx, conn.URL, conn.User, conn.Pass); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.st.SetJSON(ctx, store.KeyAdGuard, conn); err != nil {
		internalError(w, "save AdGuard connection", err)
		return
	}
	s.ag.Configure(conn.URL, conn.User, conn.Pass)

	s.mu.Lock()
	defer s.mu.Unlock()
	synced, removed, warnings, err := s.agSyncAllLocked(ctx)
	if err != nil {
		warnings = append(warnings, "device sync: "+err.Error())
	}
	writeJSON(w, http.StatusOK, map[string]any{"synced": synced, "removed": removed, "warnings": warnings})
}

func (s *Server) handleAdGuardDeleteConn(w http.ResponseWriter, r *http.Request) {
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "read server", err)
		return
	}
	if err := s.disconnectAdGuard(r.Context(), srv); err != nil {
		internalError(w, "delete AdGuard connection", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": s.ag.Enabled()})
}

func (s *Server) handleAdGuardJob(w http.ResponseWriter, r *http.Request) {
	s.agJobMu.Lock()
	job := s.agJob
	s.agJobMu.Unlock()
	if job.State == "" {
		job.State = "idle"
	}
	conn, source, err := s.currentConn(r.Context())
	if err != nil {
		internalError(w, "read AdGuard connection", err)
		return
	}
	detected := false
	if !s.ag.Enabled() {
		detected = adguard.WizardState(r.Context(), adguard.WizardURL) != "down"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"action": job.Action, "state": job.State, "message": job.Message,
		"helper_available": helper.Available(),
		"managed":          source == "db" && conn.Managed,
		"detected":         detected,
	})
}

func (s *Server) handleAdGuardInstall(w http.ResponseWriter, r *http.Request) {
	if s.ag.Enabled() {
		writeError(w, http.StatusConflict, "AdGuard is already connected")
		return
	}
	if !s.startAGJob("install", s.installAdGuard) {
		writeError(w, http.StatusConflict, "an AdGuard task is still running")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"state": "running"})
}

func (s *Server) handleAdGuardUninstall(w http.ResponseWriter, r *http.Request) {
	if !s.startAGJob("uninstall", s.uninstallAdGuard) {
		writeError(w, http.StatusConflict, "an AdGuard task is still running")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"state": "running"})
}

// ---------- per-client settings ----------

func (s *Server) handleAdGuardGetClient(w http.ResponseWriter, r *http.Request) {
	c, ok := s.clientOr404(w, r)
	if !ok || !s.agReady(w) {
		return
	}
	e, err := s.ag.GetClient(r.Context(), c.ID)
	if err != nil {
		agError(w, "read AdGuard client", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": e})
}

func (s *Server) handleAdGuardPutClient(w http.ResponseWriter, r *http.Request) {
	c, ok := s.clientOr404(w, r)
	if !ok {
		return
	}
	var in adguard.ClientSettings
	if !readJSON(w, r, &in) || !s.agReady(w) {
		return
	}
	if err := s.ag.UpdateClient(r.Context(), c.ID, in); err != nil {
		agError(w, "update AdGuard client", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- AdGuard settings proxy ----------

// agProxyRoutes lists the AdGuard /control/... endpoints that may be reached through
// the panel at /api/adguard/ctl/... (an allowlist, not an open proxy).
var agProxyRoutes = []struct{ method, path string }{
	{"GET", "filtering/status"},
	{"GET", "dns_info"},
	{"GET", "blocked_services/all"},
	{"GET", "blocked_services/get"},
	{"GET", "querylog"},
	{"GET", "querylog/config"},
	{"GET", "safesearch/status"},
	{"GET", "safebrowsing/status"},
	{"GET", "parental/status"},
	{"POST", "filtering/config"},
	{"POST", "filtering/add_url"},
	{"POST", "filtering/remove_url"},
	{"POST", "filtering/set_url"},
	{"POST", "filtering/refresh"},
	{"POST", "filtering/set_rules"},
	{"POST", "dns_config"},
	{"POST", "test_upstream_dns"},
	{"POST", "querylog_clear"},
	{"POST", "safebrowsing/enable"},
	{"POST", "safebrowsing/disable"},
	{"POST", "parental/enable"},
	{"POST", "parental/disable"},
	{"POST", "stats_reset"},
	{"PUT", "blocked_services/update"},
	{"PUT", "safesearch/settings"},
	{"PUT", "querylog/config/update"},
}

func (s *Server) agProxy(method, upstream string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.agReady(w) {
			return
		}
		path := upstream
		var body any
		if method == http.MethodGet {
			if r.URL.RawQuery != "" {
				path += "?" + r.URL.RawQuery
			}
		} else {
			b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
			if err != nil {
				writeError(w, http.StatusBadRequest, "failed to read request body")
				return
			}
			if len(b) > 1<<20 {
				writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			if len(bytes.TrimSpace(b)) > 0 {
				if !json.Valid(b) {
					writeError(w, http.StatusBadRequest, "invalid request body")
					return
				}
				body = json.RawMessage(b)
			}
		}
		raw, err := s.ag.Raw(r.Context(), method, path, body)
		if err != nil {
			agError(w, "AdGuard", err)
			return
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || string(raw) == "null" || !json.Valid(raw) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(raw)
	}
}

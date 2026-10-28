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

// agJob adalah status pemasangan/pelepasan AdGuard yang berjalan di latar belakang.
type agJob struct {
	Action  string `json:"action"`
	State   string `json:"state"` // idle | running | done | error
	Message string `json:"message"`
}

var defaultDNS = []string{"8.8.8.8", "9.9.9.9"}

// loadAdGuardConn memakai koneksi AdGuard yang disimpan di database bila ada;
// bila tidak, tetap memakai nilai dari environment.
func (s *Server) loadAdGuardConn(ctx context.Context) {
	var conn store.AdGuardConn
	found, err := s.st.GetJSON(ctx, store.KeyAdGuard, &conn)
	if err != nil {
		internalLog("baca koneksi AdGuard", err)
		return
	}
	if found && conn.URL != "" {
		s.ag.Configure(conn.URL, conn.User, conn.Pass)
	}
}

// currentConn mengembalikan koneksi yang aktif beserta asalnya: db, env, atau none.
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
		return "", fmt.Errorf("alamat server %q tidak valid", srv.Address)
	}
	return p.Addr().String(), nil
}

// ---------- pekerjaan latar belakang ----------

func (s *Server) startAGJob(action string, fn func(ctx context.Context) (string, error)) bool {
	s.agJobMu.Lock()
	if s.agJob.State == "running" {
		s.agJobMu.Unlock()
		return false
	}
	s.agJob = agJob{Action: action, State: "running", Message: "memulai..."}
	s.agJobMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		msg, err := fn(ctx)
		s.agJobMu.Lock()
		defer s.agJobMu.Unlock()
		if err != nil {
			slog.Error("AdGuard "+action+" gagal", "err", err)
			s.agJob = agJob{Action: action, State: "error", Message: err.Error()}
			return
		}
		slog.Info("AdGuard "+action+" selesai", "msg", msg)
		s.agJob = agJob{Action: action, State: "done", Message: msg}
	}()
	return true
}

func (s *Server) setAGJobMsg(msg string) {
	s.agJobMu.Lock()
	s.agJob.Message = msg
	s.agJobMu.Unlock()
}

// maybeAutoInstallAdGuard memasang AdGuard otomatis bila installer dijalankan
// dengan --with-adguard (UMBRA_ADGUARD_AUTOINSTALL=true) dan belum terhubung.
func (s *Server) maybeAutoInstallAdGuard() {
	if !s.cfg.AdGuardAutoInstall || s.ag.Enabled() {
		return
	}
	s.startAGJob("install", s.installAdGuard)
}

// runHelper meminta umbra-helper (root) mengerjakan req lalu menunggu hasilnya.
// wait (opsional) dipanggil berulang setelah helper selesai sampai bernilai true.
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
				return "", errors.New("helper tidak merespons (cek: systemctl status umbra-helper.path)")
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
			return "", errors.New("helper terlalu lama, cek: journalctl -u umbra-helper")
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
	return "", errors.New(msg + ", tetapi AdGuard belum menjawab (cek: systemctl status AdGuardHome)")
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
		return "", errors.New("atur server WireGuard dulu sebelum memasang AdGuard")
	}
	ip, err := s.tunnelIP(srv)
	if err != nil {
		return "", err
	}

	switch adguard.WizardState(ctx, adguard.WizardURL) {
	case "ready":
		return "", errors.New("AdGuard Home sudah terpasang dan disiapkan di " + adguard.WizardURL +
			"; hubungkan dengan login yang sudah ada di tab Koneksi")
	case "down":
		if !helper.Available() {
			return "", errors.New("helper pemasang belum ada di server; jalankan ulang install.sh versi terbaru, " +
				"atau pasang AdGuard Home sendiri lalu hubungkan di tab Koneksi")
		}
		s.setAGJobMsg("mengunduh dan memasang AdGuard Home...")
		_, err := s.runHelper(ctx, helper.Request{Action: "install", WGInterface: srv.Interface}, func() bool {
			return adguard.WizardState(ctx, adguard.WizardURL) == "wizard"
		})
		if err != nil {
			return "", err
		}
	}

	s.setAGJobMsg("menjalankan setup awal AdGuard...")
	pass := auth.NewToken()
	if err := adguard.RunWizard(ctx, adguard.WizardURL, ip, "admin", pass); err != nil {
		return "", fmt.Errorf("setup awal AdGuard: %w", err)
	}
	// Setelah setup, AdGuard memulai ulang web dan DNS-nya; tunggu sampai siap.
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
		return "", fmt.Errorf("AdGuard belum siap setelah setup: %w", testErr)
	}

	conn := store.AdGuardConn{URL: adguard.WizardURL, User: "admin", Pass: pass, Managed: true}
	if err := s.st.SetJSON(ctx, store.KeyAdGuard, conn); err != nil {
		return "", err
	}
	s.ag.Configure(conn.URL, conn.User, conn.Pass)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.setDefaultDNS(ctx, srv, []string{ip}); err != nil {
		internalLog("ubah DNS default", err)
	}
	synced, _, warnings, err := s.agSyncAllLocked(ctx)
	if err != nil {
		return "", fmt.Errorf("AdGuard terpasang, tetapi sinkron klien gagal: %w", err)
	}
	msg := fmt.Sprintf("AdGuard Home terpasang dan %d klien didaftarkan.", synced)
	if n := s.clientsWithoutDNS(ctx, ip); n > 0 {
		msg += fmt.Sprintf(" %d klien lama masih memakai DNS lain; unduh ulang profilnya supaya iklan ikut diblokir.", n)
	}
	for _, w := range warnings {
		msg += " Peringatan: " + w
	}
	return msg, nil
}

// setDefaultDNS mengganti DNS default klien baru. Pemanggil memegang s.mu.
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
		return "", errors.New("AdGuard ini tidak dipasang oleh panel; lepas sendiri di server, lalu putuskan koneksinya")
	}
	if !helper.Available() {
		return "", errors.New("helper pemasang belum ada di server; jalankan ulang install.sh versi terbaru")
	}
	srv, err := s.loadServer(ctx)
	if err != nil {
		return "", err
	}
	s.setAGJobMsg("melepas AdGuard Home...")
	msg, err := s.runHelper(ctx, helper.Request{Action: "uninstall", WGInterface: srv.Interface}, nil)
	if err != nil {
		return "", err
	}
	if err := s.disconnectAdGuard(ctx, srv); err != nil {
		return "", err
	}
	return msg + ". Klien yang memakai DNS AdGuard perlu mengunduh ulang profilnya.", nil
}

// disconnectAdGuard menghapus koneksi di database (kembali ke nilai environment)
// dan mengembalikan DNS default bila masih menunjuk ke AdGuard.
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

// ---------- endpoint koneksi & pemasangan ----------

func (s *Server) handleAdGuardGetConn(w http.ResponseWriter, r *http.Request) {
	conn, source, err := s.currentConn(r.Context())
	if err != nil {
		internalError(w, "baca koneksi AdGuard", err)
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

// resolveConnInput memvalidasi URL dan mengisi password lama bila kosong.
func (s *Server) resolveConnInput(ctx context.Context, in agConnInput) (store.AdGuardConn, error) {
	u, err := url.Parse(in.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return store.AdGuardConn{}, errors.New("alamat AdGuard harus berupa http:// atau https:// lengkap dengan host")
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
		internalError(w, "simpan koneksi AdGuard", err)
		return
	}
	s.ag.Configure(conn.URL, conn.User, conn.Pass)

	s.mu.Lock()
	defer s.mu.Unlock()
	synced, removed, warnings, err := s.agSyncAllLocked(ctx)
	if err != nil {
		warnings = append(warnings, "sinkron klien: "+err.Error())
	}
	writeJSON(w, http.StatusOK, map[string]any{"synced": synced, "removed": removed, "warnings": warnings})
}

func (s *Server) handleAdGuardDeleteConn(w http.ResponseWriter, r *http.Request) {
	srv, err := s.loadServer(r.Context())
	if err != nil {
		internalError(w, "baca server", err)
		return
	}
	if err := s.disconnectAdGuard(r.Context(), srv); err != nil {
		internalError(w, "hapus koneksi AdGuard", err)
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
		internalError(w, "baca koneksi AdGuard", err)
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
		writeError(w, http.StatusConflict, "AdGuard sudah terhubung")
		return
	}
	if !s.startAGJob("install", s.installAdGuard) {
		writeError(w, http.StatusConflict, "masih ada proses AdGuard yang berjalan")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"state": "running"})
}

func (s *Server) handleAdGuardUninstall(w http.ResponseWriter, r *http.Request) {
	if !s.startAGJob("uninstall", s.uninstallAdGuard) {
		writeError(w, http.StatusConflict, "masih ada proses AdGuard yang berjalan")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"state": "running"})
}

// ---------- pengaturan per klien ----------

func (s *Server) handleAdGuardGetClient(w http.ResponseWriter, r *http.Request) {
	c, ok := s.clientOr404(w, r)
	if !ok || !s.agReady(w) {
		return
	}
	e, err := s.ag.GetClient(r.Context(), c.ID)
	if err != nil {
		agError(w, "baca client AdGuard", err)
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
		agError(w, "ubah client AdGuard", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- proxy pengaturan AdGuard ----------

// agProxyRoutes: endpoint /control/... AdGuard yang boleh diakses lewat panel
// di /api/adguard/ctl/... (daftar putih, bukan proxy bebas).
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
				writeError(w, http.StatusBadRequest, "gagal membaca data")
				return
			}
			if len(b) > 1<<20 {
				writeError(w, http.StatusRequestEntityTooLarge, "data terlalu besar")
				return
			}
			if len(bytes.TrimSpace(b)) > 0 {
				if !json.Valid(b) {
					writeError(w, http.StatusBadRequest, "data tidak valid")
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

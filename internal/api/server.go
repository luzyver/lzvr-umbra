// Package api berisi HTTP API panel (JSON) dan penyajian frontend.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/luzyver/lzvr-umbra/internal/adguard"
	"github.com/luzyver/lzvr-umbra/internal/auth"
	"github.com/luzyver/lzvr-umbra/internal/config"
	"github.com/luzyver/lzvr-umbra/internal/firewall"
	"github.com/luzyver/lzvr-umbra/internal/store"
	"github.com/luzyver/lzvr-umbra/internal/wg"
)

const (
	sessionCookie = "umbra_session"
	sessionTTL    = 7 * 24 * time.Hour
	csrfHeader    = "X-Umbra-Request"
	// Lama masa percobaan firewall sebelum dikembalikan otomatis.
	firewallTrial = 60 * time.Second
	// Klien dianggap online bila handshake terakhir lebih baru dari ini.
	onlineWindow = 3 * time.Minute
)

type Server struct {
	cfg     config.Config
	st      *store.Store
	wg      wg.Backend
	fw      *firewall.Manager
	ag      *adguard.Client
	limiter *auth.Limiter

	// mu menjaga agar perubahan server/klien dan sinkronisasi ke kernel tidak saling tumpang.
	mu sync.Mutex

	setupMu     sync.Mutex
	setupToken  string // kosong setelah admin pertama dibuat
	twoFactorMu sync.Mutex
	loginSteps  map[string]loginStep
	setupSteps  map[int64]setupStep

	// agJob: pekerjaan pasang/lepas AdGuard yang sedang/terakhir berjalan.
	agJobMu sync.Mutex
	agJob   agJob
}

func New(cfg config.Config, st *store.Store, backend wg.Backend, fw *firewall.Manager, ag *adguard.Client, setupToken string) *Server {
	return &Server{
		cfg:        cfg,
		st:         st,
		wg:         backend,
		fw:         fw,
		ag:         ag,
		limiter:    auth.NewLimiter(5, 15*time.Minute),
		setupToken: setupToken,
		loginSteps: make(map[string]loginStep),
		setupSteps: make(map[int64]setupStep),
	}
}

// Handler menggabungkan API dan frontend. scriptHashes dipakai untuk CSP
// supaya skrip inline bawaan SvelteKit diizinkan tanpa 'unsafe-inline'.
func (s *Server) Handler(ui http.Handler, scriptHashes []string) http.Handler {
	mux := http.NewServeMux()
	pub := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, h) }
	priv := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, s.requireAuth(h)) }

	pub("GET /api/setup/status", s.handleSetupStatus)
	pub("POST /api/setup/admin", s.handleSetupAdmin)

	pub("POST /api/auth/login", s.handleLogin)
	pub("POST /api/auth/verify", s.handleLoginVerify)
	pub("POST /api/auth/logout", s.handleLogout)
	priv("GET /api/auth/me", s.handleMe)
	priv("POST /api/auth/password", s.handleChangePassword)
	priv("GET /api/auth/totp", s.handleTOTPStatus)
	priv("POST /api/auth/totp/start", s.handleTOTPStart)
	priv("POST /api/auth/totp/enable", s.handleTOTPEnable)
	priv("POST /api/auth/totp/disable", s.handleTOTPDisable)

	priv("GET /api/system/check", s.handleSystemCheck)

	priv("GET /api/server", s.handleGetServer)
	priv("PUT /api/server", s.handlePutServer)
	priv("GET /api/defaults", s.handleGetDefaults)
	priv("PUT /api/defaults", s.handlePutDefaults)

	priv("GET /api/firewall", s.handleGetFirewall)
	priv("POST /api/firewall/apply", s.handleApplyFirewall)
	priv("POST /api/firewall/confirm", s.handleConfirmFirewall)
	priv("POST /api/firewall/remove", s.handleRemoveFirewall)

	priv("GET /api/clients", s.handleListClients)
	priv("POST /api/clients", s.handleCreateClient)
	priv("POST /api/import/preview", s.handleImportPreview)
	priv("POST /api/import/commit", s.handleImportCommit)
	priv("POST /api/clients/{id}/regenerate-key", s.handleRegenerateClientKey)
	priv("GET /api/clients/{id}", s.handleGetClient)
	priv("PUT /api/clients/{id}", s.handleUpdateClient)
	priv("DELETE /api/clients/{id}", s.handleDeleteClient)
	priv("GET /api/clients/{id}/config", s.handleClientConfig)
	priv("GET /api/clients/{id}/qr.png", s.handleClientQR)
	priv("GET /api/clients/{id}/connections", s.handleClientConnections)

	priv("GET /api/status", s.handleStatus)
	priv("GET /api/events", s.handleEvents)
	priv("GET /api/activity", s.handleListActivity)
	priv("DELETE /api/activity", s.handleClearActivity)

	priv("GET /api/adguard/status", s.handleAdGuardStatus)
	priv("POST /api/adguard/protection", s.handleAdGuardProtection)
	priv("GET /api/adguard/stats", s.handleAdGuardStats)
	priv("POST /api/adguard/sync", s.handleAdGuardSync)
	priv("GET /api/adguard/connection", s.handleAdGuardGetConn)
	priv("PUT /api/adguard/connection", s.handleAdGuardPutConn)
	priv("DELETE /api/adguard/connection", s.handleAdGuardDeleteConn)
	priv("POST /api/adguard/connection/test", s.handleAdGuardTestConn)
	priv("GET /api/adguard/install", s.handleAdGuardJob)
	priv("POST /api/adguard/install", s.handleAdGuardInstall)
	priv("POST /api/adguard/uninstall", s.handleAdGuardUninstall)
	priv("GET /api/adguard/clients/{id}", s.handleAdGuardGetClient)
	priv("PUT /api/adguard/clients/{id}", s.handleAdGuardPutClient)
	for _, rt := range agProxyRoutes {
		priv(rt.method+" /api/adguard/ctl/"+rt.path, s.agProxy(rt.method, "/control/"+rt.path))
	}

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "endpoint tidak ada")
	})
	mux.Handle("/", ui)

	return s.securityHeaders(scriptHashes, s.csrf(mux))
}

// ---- middleware ----

type ctxKey int

const adminKey ctxKey = 1

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "belum login")
			return
		}
		id, err := s.st.SessionAdmin(r.Context(), auth.HashToken(c.Value))
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				slog.Error("cek sesi", "err", err)
			}
			writeError(w, http.StatusUnauthorized, "sesi berakhir, silakan login lagi")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminKey, id)))
	})
}

func adminID(r *http.Request) int64 {
	id, _ := r.Context().Value(adminKey).(int64)
	return id
}

// csrf: request yang mengubah data wajib membawa header khusus. Browser tidak
// mengizinkan situs lain mengirim header ini tanpa izin CORS (yang tidak kita berikan).
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get(csrfHeader) != "1" {
				writeError(w, http.StatusForbidden, "header "+csrfHeader+" wajib ada")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(scriptHashes []string, next http.Handler) http.Handler {
	scriptSrc := "'self'"
	for _, h := range scriptHashes {
		scriptSrc += " " + h // h sudah berkutip: 'sha256-...'
	}
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src " + scriptSrc,
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"connect-src 'self'",
		"font-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP hanya mempercayai header proxy bila UMBRA_TRUST_PROXY aktif.
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
			return xr
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---- helper JSON ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("tulis respons", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// internalError mencatat detail di log, tetapi tetap memberi pesan yang bisa dibaca.
func internalError(w http.ResponseWriter, what string, err error) {
	slog.Error(what, "err", err)
	writeError(w, http.StatusInternalServerError, what+": "+err.Error())
}

// internalLog untuk kegagalan yang tidak perlu menggagalkan request.
func internalLog(what string, err error) { slog.Warn(what, "err", err) }

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "data tidak valid: "+err.Error())
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		writeError(w, http.StatusBadRequest, "data tidak valid: ada isi tambahan setelah JSON")
		return false
	}
	return true
}

// writeFileAtomic menulis ke file sementara lalu rename, jadi file tidak pernah setengah jadi.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		slog.Debug("chmod file sementara", "err", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("simpan %s: %w", path, err)
	}
	return nil
}

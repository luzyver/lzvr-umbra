package api

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/luzyver/lzvr-umbra/internal/auth"
	"github.com/luzyver/lzvr-umbra/internal/store"
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

var dummyHash = sync.OnceValue(func() string {
	h, _ := auth.HashPassword(auth.NewToken())
	return h
})

// SetupTokenPath is the location of the one-time token for creating the first admin.
func SetupTokenPath(dataDir string) string { return filepath.Join(dataDir, "setup-token") }

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	n, err := s.st.AdminCount(r.Context())
	if err != nil {
		internalError(w, "read admin", err)
		return
	}
	var srv store.ServerConfig
	if _, err := s.st.GetJSON(r.Context(), store.KeyServer, &srv); err != nil {
		internalError(w, "read server config", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_admin": n == 0, "initialized": srv.Initialized})
}

func (s *Server) handleSetupAdmin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ip := s.clientIP(r)
	if ok, wait := s.limiter.Allowed(ip); !ok {
		tooMany(w, wait)
		return
	}

	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	n, err := s.st.AdminCount(r.Context())
	if err != nil {
		internalError(w, "read admin", err)
		return
	}
	if n > 0 || s.setupToken == "" {
		writeError(w, http.StatusConflict, "an admin already exists, please log in")
		return
	}
	if !auth.Equal(in.Token, s.setupToken) {
		s.limiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "incorrect setup token. See the setup-token file in the data folder or the service log")
		return
	}
	if !usernameRe.MatchString(in.Username) {
		writeError(w, http.StatusBadRequest, "username must be 3-32 characters: letters, digits, dots, underscores, or hyphens")
		return
	}
	if err := auth.ValidatePassword(in.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		internalError(w, "hash password", err)
		return
	}
	id, err := s.st.CreateAdmin(r.Context(), in.Username, hash)
	if err != nil {
		internalError(w, "create admin", err)
		return
	}
	s.setupToken = ""
	if err := os.Remove(SetupTokenPath(s.cfg.DataDir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Not fatal: the token can no longer be used because an admin already exists.
		internalLog("delete setup-token", err)
	}
	s.limiter.Reset(ip)
	if err := s.startSession(w, r, id); err != nil {
		internalError(w, "create session", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "username": in.Username})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ip := s.clientIP(r)
	if ok, wait := s.limiter.Allowed(ip); !ok {
		tooMany(w, wait)
		return
	}
	a, err := s.st.AdminByUsername(r.Context(), in.Username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		internalError(w, "read admin", err)
		return
	}
	// If the user does not exist, still check against a dummy hash so response time does not leak the username.
	hash := a.PasswordHash
	if err != nil {
		hash = dummyHash()
	}
	ok := auth.VerifyPassword(in.Password, hash) && err == nil
	if !ok {
		s.limiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "incorrect username or password")
		return
	}
	if a.TOTPSecret != "" {
		token := auth.NewToken()
		s.twoFactorMu.Lock()
		for k, v := range s.loginSteps {
			if time.Now().After(v.expires) || v.adminID == a.ID {
				delete(s.loginSteps, k)
			}
		}
		s.loginSteps[auth.HashToken(token)] = loginStep{adminID: a.ID, expires: time.Now().Add(5 * time.Minute)}
		s.twoFactorMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"requires_totp": true, "challenge": token})
		return
	}
	s.limiter.Reset(ip)
	if err := s.startSession(w, r, a.ID); err != nil {
		internalError(w, "create session", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": a.ID, "username": a.Username})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		if err := s.st.DeleteSession(r.Context(), auth.HashToken(c.Value)); err != nil {
			internalLog("delete session", err)
		}
	}
	s.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	a, err := s.st.AdminByID(r.Context(), adminID(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       a.ID,
		"username": a.Username,
		"adguard":  s.ag.Enabled(),
		"fake_wg":  s.cfg.FakeWG,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	a, err := s.st.AdminByID(r.Context(), adminID(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
		return
	}
	if !auth.VerifyPassword(in.Current, a.PasswordHash) {
		writeError(w, http.StatusBadRequest, "current password is incorrect")
		return
	}
	if err := auth.ValidatePassword(in.New); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(in.New)
	if err != nil {
		internalError(w, "hash password", err)
		return
	}
	if err := s.st.UpdateAdminPassword(r.Context(), a.ID, hash); err != nil {
		internalError(w, "save password", err)
		return
	}
	// Other sessions (other devices) are logged out; this session stays.
	c, _ := r.Cookie(sessionCookie)
	if err := s.st.DeleteAdminSessions(r.Context(), a.ID, auth.HashToken(c.Value)); err != nil {
		internalLog("delete other sessions", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, id int64) error {
	token := auth.NewToken()
	expires := time.Now().Add(sessionTTL)
	if err := s.st.CreateSession(r.Context(), auth.HashToken(token), id, expires); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

func tooMany(w http.ResponseWriter, wait time.Duration) {
	mins := int(math.Ceil(wait.Minutes()))
	w.Header().Set("Retry-After", fmt.Sprint(int(math.Ceil(wait.Seconds()))))
	writeError(w, http.StatusTooManyRequests, fmt.Sprintf("too many failed attempts, try again in %d minutes", mins))
}

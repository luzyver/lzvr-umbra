package api

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"time"

	"github.com/luzyver/lzvr-umbra/internal/auth"
	"github.com/luzyver/lzvr-umbra/internal/store"
	qrcode "github.com/skip2/go-qrcode"
)

type loginStep struct {
	adminID int64
	expires time.Time
}
type setupStep struct {
	secret  string
	expires time.Time
}

func (s *Server) handleLoginVerify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ip := s.clientIP(r)
	if ok, wait := s.limiter.Allowed(ip); !ok {
		tooMany(w, wait)
		return
	}
	s.twoFactorMu.Lock()
	defer s.twoFactorMu.Unlock()
	pending, exists := s.loginSteps[auth.HashToken(in.Challenge)]
	if !exists || time.Now().After(pending.expires) {
		s.limiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "verification expired, enter your password again")
		return
	}
	a, err := s.st.AdminByID(r.Context(), pending.adminID)
	if err != nil {
		internalError(w, "read admin", err)
		return
	}
	if a.TOTPSecret == "" {
		writeError(w, http.StatusUnauthorized, "verification is not valid")
		return
	}
	ok, err := s.consumeSecondFactor(r, a, in.Code)
	if err != nil {
		internalError(w, "two-factor verification", err)
		return
	}
	if !ok {
		s.limiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "incorrect or already used verification code")
		return
	}
	delete(s.loginSteps, auth.HashToken(in.Challenge))
	s.limiter.Reset(ip)
	if err := s.startSession(w, r, a.ID); err != nil {
		internalError(w, "create session", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": a.ID, "username": a.Username})
}

func (s *Server) consumeSecondFactor(r *http.Request, a store.Admin, code string) (bool, error) {
	if auth.ValidRecoveryCode(code) {
		return s.st.ConsumeRecoveryCode(r.Context(), a.ID, auth.HashRecoveryCode(code))
	}
	step, ok := auth.VerifyTOTP(a.TOTPSecret, code, time.Now(), a.LastTOTPStep)
	if !ok {
		return false, nil
	}
	return s.st.UseTOTPStep(r.Context(), a.ID, step)
}

func (s *Server) handleTOTPStatus(w http.ResponseWriter, r *http.Request) {
	a, err := s.st.AdminByID(r.Context(), adminID(r))
	if err != nil {
		internalError(w, "read admin", err)
		return
	}
	n, err := s.st.RecoveryCount(r.Context(), a.ID)
	if err != nil {
		internalError(w, "count recovery codes", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": a.TOTPSecret != "", "recovery_remaining": n})
}

func (s *Server) handleTOTPStart(w http.ResponseWriter, r *http.Request) {
	a, err := s.st.AdminByID(r.Context(), adminID(r))
	if err != nil {
		internalError(w, "read admin", err)
		return
	}
	if a.TOTPSecret != "" {
		writeError(w, http.StatusConflict, "two-factor authentication is already enabled")
		return
	}
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		internalError(w, "generate TOTP secret", err)
		return
	}
	uri := "otpauth://totp/" + url.PathEscape("Umbra:"+a.Username) + "?secret=" + url.QueryEscape(secret) + "&issuer=Umbra&algorithm=SHA1&digits=6&period=30"
	png, err := qrcode.Encode(uri, qrcode.Medium, 256)
	if err != nil {
		internalError(w, "generate QR code", err)
		return
	}
	s.twoFactorMu.Lock()
	s.setupSteps[a.ID] = setupStep{secret: secret, expires: time.Now().Add(10 * time.Minute)}
	s.twoFactorMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"secret": secret, "uri": uri, "qr": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)})
}

func (s *Server) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ip := s.clientIP(r)
	if ok, wait := s.limiter.Allowed(ip); !ok {
		tooMany(w, wait)
		return
	}
	a, err := s.st.AdminByID(r.Context(), adminID(r))
	if err != nil {
		internalError(w, "read admin", err)
		return
	}
	if a.TOTPSecret != "" {
		writeError(w, http.StatusConflict, "two-factor authentication is already enabled")
		return
	}
	s.twoFactorMu.Lock()
	pending, exists := s.setupSteps[a.ID]
	s.twoFactorMu.Unlock()
	if !exists || time.Now().After(pending.expires) {
		writeError(w, http.StatusBadRequest, "setup expired, start again")
		return
	}
	step, ok := auth.VerifyTOTP(pending.secret, in.Code, time.Now(), -1)
	if !ok {
		s.limiter.Fail(ip)
		writeError(w, http.StatusBadRequest, "incorrect verification code")
		return
	}
	codes, hashes, err := auth.NewRecoveryCodes()
	if err != nil {
		internalError(w, "generate recovery codes", err)
		return
	}
	if err := s.st.EnableTOTP(r.Context(), a.ID, pending.secret, step, hashes); err != nil {
		internalError(w, "enable TOTP", err)
		return
	}
	s.twoFactorMu.Lock()
	delete(s.setupSteps, a.ID)
	s.twoFactorMu.Unlock()
	s.limiter.Reset(ip)
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ip := s.clientIP(r)
	if ok, wait := s.limiter.Allowed(ip); !ok {
		tooMany(w, wait)
		return
	}
	a, err := s.st.AdminByID(r.Context(), adminID(r))
	if err != nil {
		internalError(w, "read admin", err)
		return
	}
	if a.TOTPSecret == "" {
		writeError(w, http.StatusConflict, "two-factor authentication is not enabled")
		return
	}
	if !auth.VerifyPassword(in.Password, a.PasswordHash) {
		s.limiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "incorrect password or code")
		return
	}
	ok, err := s.consumeSecondFactor(r, a, in.Code)
	if err != nil {
		internalError(w, "two-factor verification", err)
		return
	}
	if !ok {
		s.limiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "incorrect password or code")
		return
	}
	if err := s.st.DisableTOTP(r.Context(), a.ID); err != nil {
		internalError(w, "disable TOTP", err)
		return
	}
	s.limiter.Reset(ip)
	w.WriteHeader(http.StatusNoContent)
}

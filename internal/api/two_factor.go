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
		writeError(w, http.StatusUnauthorized, "verifikasi berakhir, masukkan kata sandi lagi")
		return
	}
	a, err := s.st.AdminByID(r.Context(), pending.adminID)
	if err != nil {
		internalError(w, "baca admin", err)
		return
	}
	if a.TOTPSecret == "" {
		writeError(w, http.StatusUnauthorized, "verifikasi tidak berlaku")
		return
	}
	ok, err := s.consumeSecondFactor(r, a, in.Code)
	if err != nil {
		internalError(w, "verifikasi dua langkah", err)
		return
	}
	if !ok {
		s.limiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "kode verifikasi salah atau sudah dipakai")
		return
	}
	delete(s.loginSteps, auth.HashToken(in.Challenge))
	s.limiter.Reset(ip)
	if err := s.startSession(w, r, a.ID); err != nil {
		internalError(w, "buat sesi", err)
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
		internalError(w, "baca admin", err)
		return
	}
	n, err := s.st.RecoveryCount(r.Context(), a.ID)
	if err != nil {
		internalError(w, "hitung kode pemulihan", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": a.TOTPSecret != "", "recovery_remaining": n})
}

func (s *Server) handleTOTPStart(w http.ResponseWriter, r *http.Request) {
	a, err := s.st.AdminByID(r.Context(), adminID(r))
	if err != nil {
		internalError(w, "baca admin", err)
		return
	}
	if a.TOTPSecret != "" {
		writeError(w, http.StatusConflict, "verifikasi dua langkah sudah aktif")
		return
	}
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		internalError(w, "buat kunci TOTP", err)
		return
	}
	uri := "otpauth://totp/" + url.PathEscape("Umbra:"+a.Username) + "?secret=" + url.QueryEscape(secret) + "&issuer=Umbra&algorithm=SHA1&digits=6&period=30"
	png, err := qrcode.Encode(uri, qrcode.Medium, 256)
	if err != nil {
		internalError(w, "buat QR", err)
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
		internalError(w, "baca admin", err)
		return
	}
	if a.TOTPSecret != "" {
		writeError(w, http.StatusConflict, "verifikasi dua langkah sudah aktif")
		return
	}
	s.twoFactorMu.Lock()
	pending, exists := s.setupSteps[a.ID]
	s.twoFactorMu.Unlock()
	if !exists || time.Now().After(pending.expires) {
		writeError(w, http.StatusBadRequest, "pengaturan kedaluwarsa, mulai lagi")
		return
	}
	step, ok := auth.VerifyTOTP(pending.secret, in.Code, time.Now(), -1)
	if !ok {
		s.limiter.Fail(ip)
		writeError(w, http.StatusBadRequest, "kode verifikasi salah")
		return
	}
	codes, hashes, err := auth.NewRecoveryCodes()
	if err != nil {
		internalError(w, "buat kode pemulihan", err)
		return
	}
	if err := s.st.EnableTOTP(r.Context(), a.ID, pending.secret, step, hashes); err != nil {
		internalError(w, "aktifkan TOTP", err)
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
		internalError(w, "baca admin", err)
		return
	}
	if a.TOTPSecret == "" {
		writeError(w, http.StatusConflict, "verifikasi dua langkah belum aktif")
		return
	}
	if !auth.VerifyPassword(in.Password, a.PasswordHash) {
		s.limiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "kata sandi atau kode salah")
		return
	}
	ok, err := s.consumeSecondFactor(r, a, in.Code)
	if err != nil {
		internalError(w, "verifikasi dua langkah", err)
		return
	}
	if !ok {
		s.limiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "kata sandi atau kode salah")
		return
	}
	if err := s.st.DisableTOTP(r.Context(), a.ID); err != nil {
		internalError(w, "nonaktifkan TOTP", err)
		return
	}
	s.limiter.Reset(ip)
	w.WriteHeader(http.StatusNoContent)
}

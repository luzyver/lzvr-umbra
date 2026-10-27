package api

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/luzyver/lzvr-umbra/internal/adguard"
)

var agTagRe = regexp.MustCompile(`\[wg#(\d+)\]$`)

// agReady memastikan AdGuard dikonfigurasi; bila tidak, balas 503.
func (s *Server) agReady(w http.ResponseWriter) bool {
	if !s.ag.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "AdGuard belum dihubungkan (atur UMBRA_ADGUARD_URL)")
		return false
	}
	return true
}

func agError(w http.ResponseWriter, what string, err error) {
	if errors.Is(err, adguard.ErrDisabled) {
		writeError(w, http.StatusServiceUnavailable, "AdGuard belum dihubungkan")
		return
	}
	internalLog(what, err)
	writeError(w, http.StatusBadGateway, what+": "+err.Error())
}

func (s *Server) handleAdGuardStatus(w http.ResponseWriter, r *http.Request) {
	if !s.ag.Enabled() {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	st, err := s.ag.Status(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": true, "reachable": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "reachable": true, "status": st})
}

func (s *Server) handleAdGuardProtection(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.Enabled == nil {
		writeError(w, http.StatusBadRequest, "field enabled wajib diisi")
		return
	}
	if !s.agReady(w) {
		return
	}
	if err := s.ag.SetProtection(r.Context(), *in.Enabled); err != nil {
		agError(w, "ubah proteksi AdGuard", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdGuardStats(w http.ResponseWriter, r *http.Request) {
	if !s.agReady(w) {
		return
	}
	raw, err := s.ag.Stats(r.Context())
	if err != nil {
		agError(w, "baca statistik AdGuard", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(raw)
}

// handleAdGuardSync menyamakan client AdGuard dengan database: semua klien
// didaftarkan ulang, dan client "[wg#N]" yang kliennya sudah dihapus ikut dibuang.
func (s *Server) handleAdGuardSync(w http.ResponseWriter, r *http.Request) {
	if !s.agReady(w) {
		return
	}
	ctx := r.Context()
	s.mu.Lock()
	defer s.mu.Unlock()

	clients, err := s.st.ListClients(ctx)
	if err != nil {
		internalError(w, "baca klien", err)
		return
	}
	existing, err := s.ag.ListClients(ctx)
	if err != nil {
		agError(w, "baca client AdGuard", err)
		return
	}
	known := map[int64]bool{}
	warnings := []string{}
	synced, removed := 0, 0
	for _, c := range clients {
		known[c.ID] = true
		if err := s.ag.Upsert(ctx, c.ID, c.Name, c.Address, c.Adblock); err != nil {
			warnings = append(warnings, c.Name+": "+err.Error())
			continue
		}
		synced++
	}
	for _, e := range existing {
		m := agTagRe.FindStringSubmatch(e.Name)
		if m == nil {
			continue // client buatan pengguna sendiri, jangan disentuh
		}
		id, _ := strconv.ParseInt(m[1], 10, 64)
		if known[id] {
			continue
		}
		if err := s.ag.Delete(ctx, id); err != nil {
			warnings = append(warnings, e.Name+": "+err.Error())
			continue
		}
		removed++
	}
	writeJSON(w, http.StatusOK, map[string]any{"synced": synced, "removed": removed, "warnings": warnings})
}

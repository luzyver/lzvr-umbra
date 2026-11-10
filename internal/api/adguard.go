package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/luzyver/lzvr-umbra/internal/adguard"
)

var agTagRe = regexp.MustCompile(`\[wg#(\d+)\]$`)

// agReady ensures AdGuard is configured; if not, it replies with 503.
func (s *Server) agReady(w http.ResponseWriter) bool {
	if !s.ag.Enabled() {
		writeError(w, http.StatusServiceUnavailable, adguard.ErrDisabled.Error())
		return false
	}
	return true
}

func agError(w http.ResponseWriter, what string, err error) {
	if errors.Is(err, adguard.ErrDisabled) {
		writeError(w, http.StatusServiceUnavailable, "AdGuard is not connected yet")
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
		writeError(w, http.StatusBadRequest, "the enabled field is required")
		return
	}
	if !s.agReady(w) {
		return
	}
	if err := s.ag.SetProtection(r.Context(), *in.Enabled); err != nil {
		agError(w, "update AdGuard protection", err)
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
		agError(w, "read AdGuard stats", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(raw)
}

// handleAdGuardSync syncs AdGuard clients with the database: all clients are
// re-registered, and "[wg#N]" clients whose client was deleted are removed.
func (s *Server) handleAdGuardSync(w http.ResponseWriter, r *http.Request) {
	if !s.agReady(w) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	synced, removed, warnings, err := s.agSyncAllLocked(r.Context())
	if err != nil {
		agError(w, "read AdGuard clients", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"synced": synced, "removed": removed, "warnings": warnings})
}

// agSyncAllLocked syncs all clients to AdGuard. The caller must hold s.mu.
func (s *Server) agSyncAllLocked(ctx context.Context) (synced, removed int, warnings []string, err error) {
	clients, err := s.st.ListClients(ctx)
	if err != nil {
		return 0, 0, nil, err
	}
	existing, err := s.ag.ListClients(ctx)
	if err != nil {
		return 0, 0, nil, err
	}
	known := map[int64]bool{}
	warnings = []string{}
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
			continue // a client the user created themselves; leave it alone
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
	return synced, removed, warnings, nil
}

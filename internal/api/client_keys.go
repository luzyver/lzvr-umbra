package api

import (
	"github.com/luzyver/lzvr-umbra/internal/wg"
	"net/http"
)

func (s *Server) handleRegenerateClientKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clientOr404(w, r)
	if !ok {
		return
	}
	pair, err := wg.NewKeyPair()
	if err != nil {
		internalError(w, "generate device key", err)
		return
	}
	if err = s.st.RegenerateClientKey(ctx, c.ID, pair.Private, pair.Public); err != nil {
		internalError(w, "save device key", err)
		return
	}
	c.PublicKey = pair.Public
	c.PrivateKey = pair.Private
	c.HasPrivateKey = true
	if err = s.syncLocked(ctx); err != nil {
		internalError(w, "key saved but failed to apply", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": c, "warnings": []string{"The device must scan the new profile."}})
}

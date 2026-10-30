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
		internalError(w, "buat kunci perangkat", err)
		return
	}
	if err = s.st.RegenerateClientKey(ctx, c.ID, pair.Private, pair.Public); err != nil {
		internalError(w, "simpan kunci perangkat", err)
		return
	}
	c.PublicKey = pair.Public
	c.PrivateKey = pair.Private
	c.HasPrivateKey = true
	if err = s.syncLocked(ctx); err != nil {
		internalError(w, "kunci disimpan tetapi gagal diterapkan", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": c, "warnings": []string{"Perangkat harus memindai profil baru."}})
}

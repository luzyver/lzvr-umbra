package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type clientStatus struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Address       string     `json:"address"`
	Enabled       bool       `json:"enabled"`
	Online        bool       `json:"online"`
	Endpoint      string     `json:"endpoint,omitempty"`
	RxBytes       int64      `json:"rx_bytes"`
	TxBytes       int64      `json:"tx_bytes"`
	LastHandshake *time.Time `json:"last_handshake"`
}

type statusReport struct {
	WGBackend   string         `json:"wg_backend"`
	Initialized bool           `json:"initialized"`
	Interface   string         `json:"interface"`
	Online      int            `json:"online"`
	Total       int            `json:"total"`
	RxBytes     int64          `json:"rx_bytes"`
	TxBytes     int64          `json:"tx_bytes"`
	Clients     []clientStatus `json:"clients"`
	Error       string         `json:"error,omitempty"`
	Time        time.Time      `json:"time"`
}

func (s *Server) buildStatus(ctx context.Context) (statusReport, error) {
	rep := statusReport{WGBackend: s.wg.Name(), Clients: []clientStatus{}, Time: time.Now()}
	srv, err := s.loadServer(ctx)
	if err != nil {
		return rep, err
	}
	rep.Initialized, rep.Interface = srv.Initialized, srv.Interface
	clients, err := s.st.ListClients(ctx)
	if err != nil {
		return rep, err
	}
	stats := map[string]int{}
	if srv.Initialized {
		list, err := s.wg.Stats(srv.Interface)
		if err != nil {
			// Still show the client list; the dashboard reports the interface problem.
			rep.Error = "interface " + srv.Interface + ": " + err.Error()
		}
		for i, p := range list {
			stats[p.PublicKey] = i
		}
		for _, c := range clients {
			cs := clientStatus{ID: c.ID, Name: c.Name, Address: c.Address, Enabled: c.Enabled}
			if i, ok := stats[c.PublicKey]; ok && c.Enabled {
				p := list[i]
				cs.Endpoint, cs.RxBytes, cs.TxBytes = p.Endpoint, p.RxBytes, p.TxBytes
				if !p.LastHandshake.IsZero() {
					t := p.LastHandshake
					cs.LastHandshake = &t
					cs.Online = time.Since(t) < onlineWindow
				}
			}
			if cs.Online {
				rep.Online++
			}
			rep.RxBytes += cs.RxBytes
			rep.TxBytes += cs.TxBytes
			rep.Clients = append(rep.Clients, cs)
		}
	}
	rep.Total = len(clients)
	return rep, nil
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	rep, err := s.buildStatus(r.Context())
	if err != nil {
		internalError(w, "read status", err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// handleEvents sends the status every 3 seconds via Server-Sent Events.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	// SSE connections are long-lived, so the server write deadline is disabled for this request.
	_ = rc.SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no") // so nginx does not buffer the data
	w.WriteHeader(http.StatusOK)

	send := func() error {
		rep, err := s.buildStatus(r.Context())
		if err != nil {
			rep.Error = err.Error()
		}
		b, _ := json.Marshal(rep)
		if _, err := fmt.Fprintf(w, "event: status\ndata: %s\n\n", b); err != nil {
			return err
		}
		return rc.Flush()
	}
	if send() != nil {
		return
	}
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-t.C:
			if send() != nil {
				return
			}
		}
	}
}

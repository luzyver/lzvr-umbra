package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/luzyver/lzvr-umbra/internal/store"
	"github.com/luzyver/lzvr-umbra/internal/wg"
)

const (
	// How often peer status is checked for the activity log.
	activityPoll = 15 * time.Second
	// Log entries older than this are deleted automatically.
	activityRetention = 90 * 24 * time.Hour
)

// peerState is the last known state of one client, kept in memory by the monitor.
type peerState struct {
	name     string
	online   bool
	endpoint string
	since    time.Time // session start
	seen     time.Time // last handshake
	lastRx   int64     // raw WireGuard counters from the previous check
	lastTx   int64
	rx, tx   int64 // traffic of the current session
}

// RunActivityMonitor records when clients connect, disconnect, and change IP.
// WireGuard has no "login/logout"; the handshake is the reference: a client counts as
// connected as long as its last handshake is newer than onlineWindow.
func (s *Server) RunActivityMonitor(ctx context.Context) {
	state := map[int64]*peerState{}
	t := time.NewTicker(activityPoll)
	defer t.Stop()
	var lastPurge time.Time
	for {
		if err := s.pollActivity(ctx, state); err != nil && ctx.Err() == nil {
			slog.Warn("activity log", "err", err)
		}
		if time.Since(lastPurge) > time.Hour {
			lastPurge = time.Now()
			if err := s.st.PurgeActivity(ctx, time.Now().Add(-activityRetention)); err != nil && ctx.Err() == nil {
				slog.Warn("purge activity log", "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Server) pollActivity(ctx context.Context, state map[int64]*peerState) error {
	srv, err := s.loadServer(ctx)
	if err != nil || !srv.Initialized {
		return err
	}
	clients, err := s.st.ListClients(ctx)
	if err != nil {
		return err
	}
	list, err := s.wg.Stats(srv.Interface)
	if err != nil {
		return err
	}
	stats := make(map[string]wg.PeerStat, len(list))
	for _, p := range list {
		stats[p.PublicKey] = p
	}
	now := time.Now()
	present := make(map[int64]bool, len(clients))

	for _, c := range clients {
		present[c.ID] = true
		p, found := stats[c.PublicKey]
		online := found && c.Enabled && !p.LastHandshake.IsZero() && now.Sub(p.LastHandshake) < onlineWindow

		st, known := state[c.ID]
		if !known {
			st = s.restorePeer(ctx, c.ID, p)
			state[c.ID] = st
		}
		st.name = c.Name

		// WireGuard counters are cumulative; if they shrink, the peer was recreated.
		dRx, dTx := p.RxBytes-st.lastRx, p.TxBytes-st.lastTx
		if dRx < 0 {
			dRx = p.RxBytes
		}
		if dTx < 0 {
			dTx = p.TxBytes
		}
		st.lastRx, st.lastTx = p.RxBytes, p.TxBytes

		switch {
		case online && !st.online:
			st.online, st.since, st.seen, st.endpoint = true, p.LastHandshake, p.LastHandshake, p.Endpoint
			st.rx, st.tx = 0, 0
			s.logActivity(ctx, &store.Activity{ClientID: c.ID, ClientName: c.Name, Kind: store.ActConnect,
				Endpoint: p.Endpoint, At: p.LastHandshake})
		case online:
			st.rx += dRx
			st.tx += dTx
			st.seen = p.LastHandshake
			if p.Endpoint != "" && st.endpoint != "" && endpointHost(p.Endpoint) != endpointHost(st.endpoint) {
				s.logActivity(ctx, &store.Activity{ClientID: c.ID, ClientName: c.Name, Kind: store.ActRoam,
					Endpoint: p.Endpoint, Detail: st.endpoint, At: now})
			}
			if p.Endpoint != "" {
				st.endpoint = p.Endpoint
			}
		case st.online:
			detail := ""
			switch {
			case !c.Enabled:
				detail = "device disabled"
			case !found:
				detail = "peer not on interface"
			}
			s.endSession(ctx, c.ID, st, detail, now)
		}
	}

	// Clients that were deleted while still connected.
	for id, st := range state {
		if present[id] {
			continue
		}
		if st.online {
			s.endSession(ctx, id, st, "device deleted", now)
		}
		delete(state, id)
	}
	return nil
}

// restorePeer is used when a client is seen for the first time (e.g. the panel just restarted):
// if its last event was "connect", that session is treated as still running.
// Traffic before the restart is unknown, so counting starts from now.
func (s *Server) restorePeer(ctx context.Context, clientID int64, p wg.PeerStat) *peerState {
	st := &peerState{lastRx: p.RxBytes, lastTx: p.TxBytes}
	last, err := s.st.LastActivity(ctx, clientID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			internalLog("read activity log", err)
		}
		return st
	}
	if last.Kind == store.ActConnect {
		st.online, st.since, st.seen, st.endpoint = true, last.At, last.At, last.Endpoint
		if p.LastHandshake.After(st.seen) {
			st.seen = p.LastHandshake
		}
	}
	return st
}

func (s *Server) endSession(ctx context.Context, clientID int64, st *peerState, detail string, now time.Time) {
	// Disconnect time = last handshake (when the client was last seen), not when it was detected.
	at := st.seen
	if at.IsZero() || at.After(now) {
		at = now
	}
	dur := at.Sub(st.since)
	if dur < 0 {
		dur = 0
	}
	s.logActivity(ctx, &store.Activity{ClientID: clientID, ClientName: st.name, Kind: store.ActDisconnect,
		Endpoint: st.endpoint, Detail: detail, Duration: int64(dur / time.Second),
		RxBytes: st.rx, TxBytes: st.tx, At: at})
	st.online, st.rx, st.tx = false, 0, 0
}

func (s *Server) logActivity(ctx context.Context, a *store.Activity) {
	if err := s.st.AddActivity(ctx, a); err != nil && ctx.Err() == nil {
		internalLog("save activity log", err)
	}
}

// endpointHost strips the port: a port change due to NAT does not mean a network change.
func endpointHost(ep string) string {
	if h, _, err := net.SplitHostPort(ep); err == nil {
		return h
	}
	return ep
}

type activityPage struct {
	Items []store.Activity `json:"items"`
	More  bool             `json:"more"`
}

func (s *Server) handleListActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.ActivityFilter{Limit: 50, Kind: q.Get("kind")}
	switch f.Kind {
	case "", store.ActConnect, store.ActDisconnect, store.ActRoam:
	default:
		writeError(w, http.StatusBadRequest, "unknown event kind")
		return
	}
	var err error
	if v := q.Get("client_id"); v != "" {
		if f.ClientID, err = strconv.ParseInt(v, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "invalid client_id")
			return
		}
	}
	if v := q.Get("before"); v != "" {
		if f.Before, err = strconv.ParseInt(v, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "invalid before")
			return
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			f.Limit = n
		}
	}
	want := f.Limit
	f.Limit++ // fetch one extra to know whether there is another page
	items, err := s.st.ListActivity(r.Context(), f)
	if err != nil {
		internalError(w, "read activity log", err)
		return
	}
	page := activityPage{Items: items}
	if len(items) > want {
		page.Items, page.More = items[:want], true
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleClearActivity(w http.ResponseWriter, r *http.Request) {
	var id int64
	if v := r.URL.Query().Get("client_id"); v != "" {
		var err error
		if id, err = strconv.ParseInt(v, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "invalid client_id")
			return
		}
	}
	n, err := s.st.ClearActivity(r.Context(), id)
	if err != nil {
		internalError(w, "delete activity log", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": n})
}

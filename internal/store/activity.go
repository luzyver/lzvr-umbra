package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Jenis kejadian di log aktivitas WireGuard.
const (
	ActConnect    = "connect"    // klien mulai terhubung (handshake pertama setelah offline)
	ActDisconnect = "disconnect" // klien tidak lagi handshake; membawa durasi dan trafik sesi
	ActRoam       = "roam"       // IP publik klien berganti saat masih terhubung
)

type Activity struct {
	ID         int64     `json:"id"`
	ClientID   int64     `json:"client_id"`
	ClientName string    `json:"client_name"`
	Kind       string    `json:"kind"`
	Endpoint   string    `json:"endpoint,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	Duration   int64     `json:"duration"` // detik, hanya untuk disconnect
	RxBytes    int64     `json:"rx_bytes"`
	TxBytes    int64     `json:"tx_bytes"`
	At         time.Time `json:"at"`
}

type ActivityFilter struct {
	ClientID int64
	Kind     string
	Before   int64 // id; 0 = dari yang terbaru
	Limit    int
}

func (s *Store) AddActivity(ctx context.Context, a *Activity) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO activity (client_id, client_name, kind, endpoint, detail, duration, rx_bytes, tx_bytes, at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ClientID, a.ClientName, a.Kind, a.Endpoint, a.Detail, a.Duration, a.RxBytes, a.TxBytes, a.At.Unix())
	if err != nil {
		return err
	}
	a.ID, err = res.LastInsertId()
	return err
}

const activityCols = `id, client_id, client_name, kind, endpoint, detail, duration, rx_bytes, tx_bytes, at`

func scanActivity(row scanner) (Activity, error) {
	var a Activity
	var at int64
	err := row.Scan(&a.ID, &a.ClientID, &a.ClientName, &a.Kind, &a.Endpoint, &a.Detail,
		&a.Duration, &a.RxBytes, &a.TxBytes, &at)
	a.At = time.Unix(at, 0)
	return a, err
}

// ListActivity mengembalikan kejadian terbaru dulu, mulai sebelum f.Before.
func (s *Store) ListActivity(ctx context.Context, f ActivityFilter) ([]Activity, error) {
	q := `SELECT ` + activityCols + ` FROM activity WHERE 1 = 1`
	args := []any{}
	if f.ClientID > 0 {
		q += ` AND client_id = ?`
		args = append(args, f.ClientID)
	}
	if f.Kind != "" {
		q += ` AND kind = ?`
		args = append(args, f.Kind)
	}
	if f.Before > 0 {
		q += ` AND id < ?`
		args = append(args, f.Before)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, f.Limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Activity{}
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LastActivity: kejadian connect/disconnect terakhir milik klien (dipakai saat panel baru menyala).
func (s *Store) LastActivity(ctx context.Context, clientID int64) (Activity, error) {
	a, err := scanActivity(s.db.QueryRowContext(ctx,
		`SELECT `+activityCols+` FROM activity WHERE client_id = ? AND kind IN (?, ?) ORDER BY id DESC LIMIT 1`,
		clientID, ActConnect, ActDisconnect))
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// ClearActivity menghapus log; clientID 0 = semua klien.
func (s *Store) ClearActivity(ctx context.Context, clientID int64) (int64, error) {
	q, args := `DELETE FROM activity`, []any{}
	if clientID > 0 {
		q += ` WHERE client_id = ?`
		args = append(args, clientID)
	}
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) PurgeActivity(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM activity WHERE at < ?`, before.Unix())
	return err
}

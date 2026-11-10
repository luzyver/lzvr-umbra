// Package store keeps all panel data in SQLite. The database is the single
// source of truth; the WireGuard configuration is always rebuilt from it.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	// the database holds private keys: only the owner may read it
	_ = os.Chmod(path, 0o600)
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS admins (
	id            INTEGER PRIMARY KEY,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	totp_secret   TEXT,
	created_at    INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	admin_id   INTEGER NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS totp_state (
	admin_id INTEGER PRIMARY KEY REFERENCES admins(id) ON DELETE CASCADE,
	last_step INTEGER NOT NULL DEFAULT -1
);
CREATE TABLE IF NOT EXISTS recovery_codes (
	code_hash TEXT PRIMARY KEY,
	admin_id INTEGER NOT NULL REFERENCES admins(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS recovery_codes_admin ON recovery_codes (admin_id);
CREATE TABLE IF NOT EXISTS clients (
	id            INTEGER PRIMARY KEY,
	name          TEXT NOT NULL,
	private_key   TEXT NOT NULL,
	public_key    TEXT NOT NULL UNIQUE,
	preshared_key TEXT NOT NULL,
	address       TEXT NOT NULL UNIQUE,
	allowed_ips   TEXT NOT NULL,
	dns           TEXT NOT NULL,
	mtu           INTEGER NOT NULL DEFAULT 0,
	keepalive     INTEGER NOT NULL DEFAULT 0,
	enabled       INTEGER NOT NULL DEFAULT 1,
	adblock       INTEGER NOT NULL DEFAULT 1,
	created_at    INTEGER NOT NULL,
	updated_at    INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS activity (
	id          INTEGER PRIMARY KEY,
	client_id   INTEGER NOT NULL,
	client_name TEXT NOT NULL,
	kind        TEXT NOT NULL,
	endpoint    TEXT NOT NULL DEFAULT '',
	detail      TEXT NOT NULL DEFAULT '',
	duration    INTEGER NOT NULL DEFAULT 0,
	rx_bytes    INTEGER NOT NULL DEFAULT 0,
	tx_bytes    INTEGER NOT NULL DEFAULT 0,
	at          INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS activity_client ON activity (client_id, id);
CREATE INDEX IF NOT EXISTS activity_at ON activity (at);`)
	return err
}

// ---- settings ----

func (s *Store) GetJSON(ctx context.Context, key string, v any) (bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), v)
}

func (s *Store) SetJSON(ctx context.Context, key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, string(raw))
	return err
}

// ---- admins & sessions ----

func (s *Store) AdminCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admins`).Scan(&n)
	return n, err
}

func (s *Store) CreateAdmin(ctx context.Context, username, hash string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO admins (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, hash, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) AdminByUsername(ctx context.Context, username string) (Admin, error) {
	var a Admin
	err := s.db.QueryRowContext(ctx,
		`SELECT a.id, a.username, a.password_hash, COALESCE(a.totp_secret, ''), COALESCE(t.last_step, -1) FROM admins a LEFT JOIN totp_state t ON t.admin_id = a.id WHERE a.username = ?`, username).
		Scan(&a.ID, &a.Username, &a.PasswordHash, &a.TOTPSecret, &a.LastTOTPStep)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func (s *Store) AdminByID(ctx context.Context, id int64) (Admin, error) {
	var a Admin
	err := s.db.QueryRowContext(ctx,
		`SELECT a.id, a.username, a.password_hash, COALESCE(a.totp_secret, ''), COALESCE(t.last_step, -1) FROM admins a LEFT JOIN totp_state t ON t.admin_id = a.id WHERE a.id = ?`, id).
		Scan(&a.ID, &a.Username, &a.PasswordHash, &a.TOTPSecret, &a.LastTOTPStep)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func (s *Store) UpdateAdminPassword(ctx context.Context, id int64, hash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admins SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

func (s *Store) CreateSession(ctx context.Context, tokenHash string, adminID int64, expires time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, admin_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash, adminID, expires.Unix())
	return err
}

// SessionAdmin returns the admin who owns a still-valid session.
func (s *Store) SessionAdmin(ctx context.Context, tokenHash string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT admin_id FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		tokenHash, time.Now().Unix()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteAdminSessions(ctx context.Context, adminID int64, except string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE admin_id = ? AND token_hash <> ?`, adminID, except)
	return err
}

func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, time.Now().Unix())
	return err
}

// ---- clients ----

const clientCols = `id, name, private_key, public_key, preshared_key, address, allowed_ips, dns,
	mtu, keepalive, enabled, adblock, created_at, updated_at`

type scanner interface{ Scan(dest ...any) error }

func scanClient(row scanner) (Client, error) {
	var c Client
	var allowed, dns string
	var enabled, adblock int
	var created, updated int64
	err := row.Scan(&c.ID, &c.Name, &c.PrivateKey, &c.PublicKey, &c.PresharedKey, &c.Address,
		&allowed, &dns, &c.MTU, &c.Keepalive, &enabled, &adblock, &created, &updated)
	if err != nil {
		return c, err
	}
	c.AllowedIPs = splitList(allowed)
	c.DNS = splitList(dns)
	c.Enabled = enabled == 1
	c.Adblock = adblock == 1
	c.HasPrivateKey = c.PrivateKey != ""
	c.CreatedAt = time.Unix(created, 0)
	c.UpdatedAt = time.Unix(updated, 0)
	return c, nil
}

func (s *Store) ListClients(ctx context.Context) ([]Client, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+clientCols+` FROM clients ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Client{}
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetClient(ctx context.Context, id int64) (Client, error) {
	c, err := scanClient(s.db.QueryRowContext(ctx, `SELECT `+clientCols+` FROM clients WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func (s *Store) CreateClient(ctx context.Context, c *Client) error {
	now := time.Now()
	res, err := s.db.ExecContext(ctx, `INSERT INTO clients
		(name, private_key, public_key, preshared_key, address, allowed_ips, dns, mtu, keepalive, enabled, adblock, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.PrivateKey, c.PublicKey, c.PresharedKey, c.Address,
		joinList(c.AllowedIPs), joinList(c.DNS), c.MTU, c.Keepalive,
		boolInt(c.Enabled), boolInt(c.Adblock), now.Unix(), now.Unix())
	if err != nil {
		return fmt.Errorf("save client: %w", err)
	}
	c.ID, _ = res.LastInsertId()
	c.CreatedAt, c.UpdatedAt = now, now
	return nil
}

func (s *Store) UpdateClient(ctx context.Context, c *Client) error {
	c.UpdatedAt = time.Now()
	res, err := s.db.ExecContext(ctx, `UPDATE clients SET
		name = ?, allowed_ips = ?, dns = ?, mtu = ?, keepalive = ?, enabled = ?, adblock = ?, updated_at = ?
		WHERE id = ?`,
		c.Name, joinList(c.AllowedIPs), joinList(c.DNS), c.MTU, c.Keepalive,
		boolInt(c.Enabled), boolInt(c.Adblock), c.UpdatedAt.Unix(), c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteClient(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM clients WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func splitList(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func joinList(l []string) string { return strings.Join(l, ",") }

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

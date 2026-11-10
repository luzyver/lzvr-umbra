package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// ImportClients saves the server and devices in a single transaction.
func (s *Store) ImportClients(ctx context.Context, clients []Client, server *ServerConfig) ([]Client, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if server != nil {
		b, e := json.Marshal(server)
		if e != nil {
			return nil, e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO settings (key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, KeyServer, string(b)); e != nil {
			return nil, e
		}
	}
	now := time.Now()
	for i := range clients {
		c := &clients[i]
		if c.CreatedAt.IsZero() {
			c.CreatedAt = now
		}
		if c.UpdatedAt.IsZero() {
			c.UpdatedAt = now
		}
		var res sql.Result
		res, err = tx.ExecContext(ctx, `INSERT INTO clients (name,private_key,public_key,preshared_key,address,allowed_ips,dns,mtu,keepalive,enabled,adblock,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.Name, c.PrivateKey, c.PublicKey, c.PresharedKey, c.Address, joinList(c.AllowedIPs), joinList(c.DNS), c.MTU, c.Keepalive, boolInt(c.Enabled), boolInt(c.Adblock), c.CreatedAt.Unix(), c.UpdatedAt.Unix())
		if err != nil {
			return nil, err
		}
		c.ID, _ = res.LastInsertId()
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return clients, nil
}

func (s *Store) RegenerateClientKey(ctx context.Context, id int64, private, public string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE clients SET private_key=?,public_key=?,updated_at=? WHERE id=?`, private, public, time.Now().Unix(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

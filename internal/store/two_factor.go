package store

import (
	"context"
	"crypto/subtle"
	"errors"
)

func (s *Store) EnableTOTP(ctx context.Context, id int64, secret string, step int64, hashes []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `UPDATE admins SET totp_secret = ? WHERE id = ? AND (totp_secret IS NULL OR totp_secret = '')`, secret, id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("verifikasi dua langkah sudah aktif")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO totp_state (admin_id, last_step) VALUES (?, ?) ON CONFLICT(admin_id) DO UPDATE SET last_step = excluded.last_step`, id, step); err != nil {
		return err
	}
	for _, hash := range hashes {
		if _, err = tx.ExecContext(ctx, `INSERT INTO recovery_codes (code_hash, admin_id) VALUES (?, ?)`, hash, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UseTOTPStep(ctx context.Context, id int64, step int64) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE totp_state SET last_step = ? WHERE admin_id = ? AND last_step < ?`, step, id, step)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n == 1, err
}

func (s *Store) ConsumeRecoveryCode(ctx context.Context, id int64, hash string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT code_hash FROM recovery_codes WHERE admin_id = ?`, id)
	if err != nil {
		return false, err
	}
	matched := ""
	for rows.Next() {
		var stored string
		if err := rows.Scan(&stored); err != nil {
			rows.Close()
			return false, err
		}
		if subtle.ConstantTimeCompare([]byte(hash), []byte(stored)) == 1 {
			matched = stored
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	if matched == "" {
		return false, nil
	}
	r, err := tx.ExecContext(ctx, `DELETE FROM recovery_codes WHERE admin_id = ? AND code_hash = ?`, id, matched)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n == 1, nil
}

func (s *Store) RecoveryCount(ctx context.Context, id int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM recovery_codes WHERE admin_id = ?`, id).Scan(&n)
	return n, err
}

func (s *Store) DisableTOTP(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE admins SET totp_secret = NULL WHERE id = ?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM totp_state WHERE admin_id = ?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM recovery_codes WHERE admin_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

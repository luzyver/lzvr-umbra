package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/luzyver/lzvr-umbra/internal/auth"
)

func TestRecoveryCodeConsumedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "umbra.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	id, err := s.CreateAdmin(ctx, "admin", "hash")
	if err != nil {
		t.Fatal(err)
	}
	codes, hashes, err := auth.NewRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnableTOTP(ctx, id, "SECRET", 100, hashes); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ConsumeRecoveryCode(ctx, id, auth.HashRecoveryCode(codes[0])); err != nil || !ok {
		t.Fatalf("first use: %v, %v", ok, err)
	}
	if ok, err := s.ConsumeRecoveryCode(ctx, id, auth.HashRecoveryCode(codes[0])); err != nil || ok {
		t.Fatalf("code reused: %v, %v", ok, err)
	}
	if ok, err := s.UseTOTPStep(ctx, id, 100); err != nil || ok {
		t.Fatalf("old step accepted: %v, %v", ok, err)
	}
	if ok, err := s.UseTOTPStep(ctx, id, 101); err != nil || !ok {
		t.Fatalf("new step rejected: %v, %v", ok, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.AdminByID(ctx, id)
	if err != nil || a.LastTOTPStep != 101 {
		t.Fatalf("step did not persist after reopen: %d, %v", a.LastTOTPStep, err)
	}
	if ok, err := s.ConsumeRecoveryCode(ctx, id, auth.HashRecoveryCode(codes[0])); err != nil || ok {
		t.Fatalf("code restored after reopen: %v, %v", ok, err)
	}
}

package auth

import "testing"

func TestRecoveryCodes(t *testing.T) {
	codes, hashes, err := NewRecoveryCodes()
	if err != nil || len(codes) != 8 || len(hashes) != 8 {
		t.Fatalf("hasil kode: %d, %d, %v", len(codes), len(hashes), err)
	}
	seen := map[string]bool{}
	for i, code := range codes {
		if seen[code] || code == hashes[i] || !ValidRecoveryCode(code) || HashRecoveryCode(code) != hashes[i] {
			t.Fatalf("kode tidak aman: %q", code)
		}
		seen[code] = true
	}
	if ValidRecoveryCode("invalid") {
		t.Fatal("format salah diterima")
	}
}

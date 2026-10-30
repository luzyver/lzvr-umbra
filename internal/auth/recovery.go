package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func NewRecoveryCodes() ([]string, []string, error) {
	codes, hashes := make([]string, 0, 8), make([]string, 0, 8)
	for range 8 {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		s := strings.ToUpper(hex.EncodeToString(b))
		code := s[:8] + "-" + s[8:16] + "-" + s[16:24] + "-" + s[24:]
		codes, hashes = append(codes, code), append(hashes, HashRecoveryCode(code))
	}
	return codes, hashes, nil
}

func ValidRecoveryCode(code string) bool {
	if len(code) != 35 {
		return false
	}
	for i, c := range code {
		if i == 8 || i == 17 || i == 26 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func HashRecoveryCode(code string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

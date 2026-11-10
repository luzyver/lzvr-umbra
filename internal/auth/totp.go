package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func NewTOTPSecret() (string, error) {
	key := make([]byte, 20)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return totpEncoding.EncodeToString(key), nil
}

func TOTPCode(secret string, step int64) (string, error) {
	key, err := totpEncoding.DecodeString(secret)
	if err != nil || len(key) == 0 || step < 0 {
		return "", errors.New("invalid TOTP key")
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := int(sum[len(sum)-1] & 15)
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1000000), nil
}

// VerifyTOTP returns the matching step only if it has not been used before.
func VerifyTOTP(secret, code string, now time.Time, lastStep int64) (int64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	current := now.Unix() / 30
	var matched int64
	valid := 0
	for step := current - 1; step <= current+1; step++ {
		if step <= lastStep || step < 0 {
			continue
		}
		want, err := TOTPCode(secret, step)
		if err != nil {
			return 0, false
		}
		match := subtle.ConstantTimeCompare([]byte(code), []byte(want))
		if match == 1 {
			matched = step
		}
		valid |= match
	}
	return matched, valid == 1
}

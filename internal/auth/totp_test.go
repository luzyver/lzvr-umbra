package auth

import (
	"encoding/base32"
	"testing"
	"time"
)

func TestTOTPVectorRFC6238(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	for _, tc := range []struct {
		unix int64
		want string
	}{
		{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"},
		{1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"},
	} {
		got, err := TOTPCode(secret, tc.unix/30)
		if err != nil || got != tc.want {
			t.Errorf("time %d: code %q, error %v; want %q", tc.unix, got, err, tc.want)
		}
	}
}

func TestTOTPWindowAndReplay(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	now := time.Unix(3000, 0)
	previous, _ := TOTPCode(secret, 99)
	step, ok := VerifyTOTP(secret, previous, now, -1)
	if !ok || step != 99 {
		t.Fatalf("previous step rejected: %d, %v", step, ok)
	}
	if _, ok := VerifyTOTP(secret, previous, now, step); ok {
		t.Fatal("code reused")
	}
	next, _ := TOTPCode(secret, 101)
	if _, ok := VerifyTOTP(secret, next, now, step); !ok {
		t.Fatal("next step rejected")
	}
	far, _ := TOTPCode(secret, 102)
	if _, ok := VerifyTOTP(secret, far, now, -1); ok {
		t.Fatal("code outside window accepted")
	}
}

// Package config membaca pengaturan proses dari environment variable.
package config

import (
	"os"
	"runtime"
	"strconv"
)

type Config struct {
	Listen       string // alamat HTTP panel, mis. 127.0.0.1:8967
	DataDir      string // tempat database, token setup, cadangan
	Dev          bool   // mode pengembangan: cookie tanpa Secure, WG palsu bila bukan Linux
	FakeWG       bool   // pakai backend WireGuard tiruan (untuk dev di Windows/macOS)
	WGBackend    string // auto, kernel, atau userspace
	FakeFirewall bool   // jangan jalankan nft sungguhan
	SecureCookie bool
	TrustProxy   bool // percayai X-Real-IP / X-Forwarded-For dari nginx

	AdGuardURL  string // mis. http://127.0.0.1:3000 (kosong = integrasi mati)
	AdGuardUser string
	AdGuardPass string
	// AdGuardAutoInstall: pasang AdGuard Home otomatis setelah WireGuard diatur
	// (diisi installer bila dijalankan dengan --with-adguard).
	AdGuardAutoInstall bool
}

func Load() Config {
	dev := envBool("UMBRA_DEV", false)
	fake := envBool("UMBRA_FAKE_WG", dev && runtime.GOOS != "linux")

	dataDir := "/var/lib/umbra"
	if dev {
		dataDir = "./data"
	}

	return Config{
		Listen:       env("UMBRA_LISTEN", "127.0.0.1:8967"),
		DataDir:      env("UMBRA_DATA_DIR", dataDir),
		Dev:          dev,
		FakeWG:       fake,
		WGBackend:    env("UMBRA_WG_BACKEND", "auto"),
		FakeFirewall: envBool("UMBRA_FAKE_FIREWALL", fake),
		SecureCookie: envBool("UMBRA_SECURE_COOKIE", !dev),
		TrustProxy:   envBool("UMBRA_TRUST_PROXY", false),
		AdGuardURL:   env("UMBRA_ADGUARD_URL", ""),
		AdGuardUser:  env("UMBRA_ADGUARD_USER", ""),
		AdGuardPass:  env("UMBRA_ADGUARD_PASS", ""),

		AdGuardAutoInstall: envBool("UMBRA_ADGUARD_AUTOINSTALL", false),
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// Package config reads process settings from environment variables.
package config

import (
	"os"
	"runtime"
	"strconv"
)

type Config struct {
	Listen       string // panel HTTP address, e.g. 127.0.0.1:8967
	DataDir      string // where the database, setup token, and backups live
	Dev          bool   // development mode: cookies without Secure, fake WG when not on Linux
	FakeWG       bool   // use the fake WireGuard backend (for dev on Windows/macOS)
	WGBackend    string // auto, kernel, or userspace
	FakeFirewall bool   // do not run the real nft
	SecureCookie bool
	TrustProxy   bool // trust X-Real-IP / X-Forwarded-For from nginx

	AdGuardURL  string // e.g. http://127.0.0.1:3000 (empty = integration disabled)
	AdGuardUser string
	AdGuardPass string
	// AdGuardAutoInstall installs AdGuard Home automatically after WireGuard is set up
	// (set by the installer when run with --with-adguard).
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

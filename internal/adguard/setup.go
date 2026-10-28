package adguard

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// Alamat web AdGuard yang dipakai saat dipasang oleh panel.
// Hanya terbuka di localhost; akses dari luar lewat panel.
const (
	WizardIP   = "127.0.0.1"
	WizardPort = 3000
	WizardURL  = "http://127.0.0.1:3000"
)

// WizardState memeriksa AdGuard di base: "wizard" bila belum disiapkan
// (setup awal belum dijalankan), "ready" bila sudah, "down" bila tidak menjawab.
func WizardState(ctx context.Context, base string) string {
	hc := &http.Client{Timeout: 3 * time.Second}
	base = strings.TrimRight(base, "/")
	err := request(ctx, hc, http.MethodGet, base+"/control/install/get_addresses", "", "", nil, nil)
	if err == nil {
		return "wizard"
	}
	if _, ok := err.(*HTTPError); ok {
		// Setelah setup selesai, endpoint wizard tidak ada lagi (404/403/302).
		return "ready"
	}
	return "down"
}

// RunWizard menjalankan setup awal AdGuard: alamat web, alamat DNS, dan akun admin.
func RunWizard(ctx context.Context, base, dnsIP, user, pass string) error {
	hc := &http.Client{Timeout: 30 * time.Second}
	body := map[string]any{
		"web":      map[string]any{"ip": WizardIP, "port": WizardPort},
		"dns":      map[string]any{"ip": dnsIP, "port": 53},
		"username": user,
		"password": pass,
	}
	return request(ctx, hc, http.MethodPost, strings.TrimRight(base, "/")+"/control/install/configure", "", "", body, nil)
}

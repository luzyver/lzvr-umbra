package adguard

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// AdGuard web address used when the panel installs it.
// It is only exposed on localhost; external access goes through the panel.
const (
	WizardIP   = "127.0.0.1"
	WizardPort = 3000
	WizardURL  = "http://127.0.0.1:3000"
)

// WizardState checks AdGuard at base: "wizard" if it is not configured yet
// (initial setup has not run), "ready" if it is, "down" if it does not respond.
func WizardState(ctx context.Context, base string) string {
	hc := &http.Client{Timeout: 3 * time.Second}
	base = strings.TrimRight(base, "/")
	err := request(ctx, hc, http.MethodGet, base+"/control/install/get_addresses", "", "", nil, nil)
	if err == nil {
		return "wizard"
	}
	if _, ok := err.(*HTTPError); ok {
		// After setup finishes, the wizard endpoint no longer exists (404/403/302).
		return "ready"
	}
	return "down"
}

// RunWizard runs the initial AdGuard setup: web address, DNS address, and admin account.
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

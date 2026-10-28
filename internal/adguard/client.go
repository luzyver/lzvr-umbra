// Package adguard berbicara dengan AdGuard Home lewat REST API-nya (/control/...).
// AdGuard berjalan sebagai service terpisah; panel hanya mengendalikannya.
package adguard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ErrDisabled = errors.New("AdGuard belum dihubungkan (pasang atau hubungkan dari halaman AdGuard)")

type Client struct {
	mu   sync.RWMutex
	base string
	user string
	pass string
	http *http.Client
}

func New(base, user, pass string) *Client {
	c := &Client{http: &http.Client{Timeout: 10 * time.Second}}
	c.Configure(base, user, pass)
	return c
}

// Configure mengganti alamat dan login AdGuard tanpa perlu restart panel.
// base kosong berarti integrasi dimatikan.
func (c *Client) Configure(base, user, pass string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.base = strings.TrimRight(strings.TrimSpace(base), "/")
	c.user = user
	c.pass = pass
}

func (c *Client) Enabled() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.base != ""
}

// URL mengembalikan alamat AdGuard yang sedang dipakai.
func (c *Client) URL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.base
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	c.mu.RLock()
	base, user, pass := c.base, c.user, c.pass
	c.mu.RUnlock()
	if base == "" {
		return ErrDisabled
	}
	return request(ctx, c.http, method, base+path, user, pass, in, out)
}

func request(ctx context.Context, hc *http.Client, method, url, user, pass string, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("AdGuard tidak bisa dihubungi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &HTTPError{Method: method, Path: req.URL.Path, Code: resp.StatusCode, Body: strings.TrimSpace(string(msg))}
	}
	if out == nil {
		return nil
	}
	if raw, ok := out.(*json.RawMessage); ok {
		b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(b)) == 0 {
			b = []byte("null")
		}
		*raw = b
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// HTTPError: AdGuard menjawab dengan status gagal.
type HTTPError struct {
	Method, Path string
	Code         int
	Body         string
}

func (e *HTTPError) Error() string {
	if e.Code == http.StatusUnauthorized || e.Code == http.StatusForbidden {
		return fmt.Sprintf("AdGuard menolak login (%d): username atau password salah", e.Code)
	}
	return fmt.Sprintf("AdGuard %s %s: %d %s", e.Method, e.Path, e.Code, e.Body)
}

// Raw meneruskan request apa adanya; dipakai untuk halaman pengaturan supaya
// panel tidak perlu meniru setiap struktur data AdGuard.
func (c *Client) Raw(ctx context.Context, method, path string, in any) (json.RawMessage, error) {
	var raw json.RawMessage
	err := c.do(ctx, method, path, in, &raw)
	return raw, err
}

type Status struct {
	Running           bool     `json:"running"`
	ProtectionEnabled bool     `json:"protection_enabled"`
	Version           string   `json:"version"`
	DNSAddresses      []string `json:"dns_addresses"`
	DNSPort           int      `json:"dns_port"`
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	var s Status
	err := c.do(ctx, http.MethodGet, "/control/status", nil, &s)
	return s, err
}

// Test mencoba alamat dan login tertentu tanpa mengubah konfigurasi yang aktif.
func (c *Client) Test(ctx context.Context, base, user, pass string) (Status, error) {
	var s Status
	err := request(ctx, c.http, http.MethodGet, strings.TrimRight(strings.TrimSpace(base), "/")+"/control/status", user, pass, nil, &s)
	return s, err
}

func (c *Client) SetProtection(ctx context.Context, enabled bool) error {
	return c.do(ctx, http.MethodPost, "/control/protection", map[string]any{"enabled": enabled}, nil)
}

// Stats dikembalikan apa adanya supaya frontend bisa memakai field yang dibutuhkan.
func (c *Client) Stats(ctx context.Context) (json.RawMessage, error) {
	return c.Raw(ctx, http.MethodGet, "/control/stats", nil)
}

// Persistent client AdGuard: satu per klien WireGuard, dikenali lewat IP tunnel.
type PersistentClient struct {
	Name                     string   `json:"name"`
	IDs                      []string `json:"ids"`
	UseGlobalSettings        bool     `json:"use_global_settings"`
	FilteringEnabled         bool     `json:"filtering_enabled"`
	ParentalEnabled          bool     `json:"parental_enabled"`
	SafebrowsingEnabled      bool     `json:"safebrowsing_enabled"`
	UseGlobalBlockedServices bool     `json:"use_global_blocked_services"`
	Tags                     []string `json:"tags"`
	Upstreams                []string `json:"upstreams"`
}

type clientsResp struct {
	Clients []PersistentClient `json:"clients"`
}

func (c *Client) ListClients(ctx context.Context) ([]PersistentClient, error) {
	var r clientsResp
	err := c.do(ctx, http.MethodGet, "/control/clients", nil, &r)
	return r.Clients, err
}

// rawClients membaca client AdGuard lengkap dengan semua field-nya, supaya
// pembaruan tidak menghapus pengaturan yang tidak dikenal panel.
func (c *Client) rawClients(ctx context.Context) ([]map[string]any, error) {
	var r struct {
		Clients []map[string]any `json:"clients"`
	}
	err := c.do(ctx, http.MethodGet, "/control/clients", nil, &r)
	return r.Clients, err
}

// ClientName: penanda [wg#id] membuat panel bisa mengenali client buatannya sendiri.
func ClientName(name string, id int64) string {
	return fmt.Sprintf("%s [wg#%d]", name, id)
}

func Tag(id int64) string { return fmt.Sprintf("[wg#%d]", id) }

func (c *Client) findClient(ctx context.Context, id int64) (map[string]any, error) {
	existing, err := c.rawClients(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range existing {
		if name, _ := e["name"].(string); strings.HasSuffix(name, Tag(id)) {
			return e, nil
		}
	}
	return nil, nil
}

// Upsert membuat atau memperbarui client AdGuard untuk satu klien WireGuard.
// Pengaturan tambahan per klien (safe search, layanan diblokir, dll.) dipertahankan.
func (c *Client) Upsert(ctx context.Context, id int64, name, ip string, adblock bool) error {
	e, err := c.findClient(ctx, id)
	if err != nil {
		return err
	}
	if e == nil {
		pc := PersistentClient{
			Name:                     ClientName(name, id),
			IDs:                      []string{ip},
			FilteringEnabled:         adblock,
			SafebrowsingEnabled:      adblock,
			UseGlobalBlockedServices: true,
			Tags:                     []string{},
			Upstreams:                []string{},
		}
		return c.do(ctx, http.MethodPost, "/control/clients/add", pc, nil)
	}
	old, _ := e["name"].(string)
	e["name"] = ClientName(name, id)
	e["ids"] = []string{ip}
	e["use_global_settings"] = false
	e["filtering_enabled"] = adblock
	return c.do(ctx, http.MethodPost, "/control/clients/update", map[string]any{"name": old, "data": e}, nil)
}

// ClientSettings adalah pengaturan per klien yang bisa diubah dari panel.
type ClientSettings struct {
	SafebrowsingEnabled      *bool    `json:"safebrowsing_enabled,omitempty"`
	ParentalEnabled          *bool    `json:"parental_enabled,omitempty"`
	SafeSearchEnabled        *bool    `json:"safe_search_enabled,omitempty"`
	UseGlobalBlockedServices *bool    `json:"use_global_blocked_services,omitempty"`
	BlockedServices          []string `json:"blocked_services,omitempty"`
	Upstreams                []string `json:"upstreams,omitempty"`
}

// GetClient mengembalikan client AdGuard mentah untuk klien WireGuard id (nil bila belum ada).
func (c *Client) GetClient(ctx context.Context, id int64) (map[string]any, error) {
	return c.findClient(ctx, id)
}

// UpdateClient menggabungkan pengaturan ke client AdGuard yang sudah ada.
func (c *Client) UpdateClient(ctx context.Context, id int64, s ClientSettings) error {
	e, err := c.findClient(ctx, id)
	if err != nil {
		return err
	}
	if e == nil {
		return errors.New("klien belum terdaftar di AdGuard, jalankan sinkron dulu")
	}
	e["use_global_settings"] = false
	if s.SafebrowsingEnabled != nil {
		e["safebrowsing_enabled"] = *s.SafebrowsingEnabled
	}
	if s.ParentalEnabled != nil {
		e["parental_enabled"] = *s.ParentalEnabled
	}
	if s.SafeSearchEnabled != nil {
		ss, _ := e["safe_search"].(map[string]any)
		if ss == nil {
			ss = defaultSafeSearch()
		}
		ss["enabled"] = *s.SafeSearchEnabled
		e["safe_search"] = ss
		e["safesearch_enabled"] = *s.SafeSearchEnabled // AdGuard versi lama
	}
	if s.UseGlobalBlockedServices != nil {
		e["use_global_blocked_services"] = *s.UseGlobalBlockedServices
	}
	if s.BlockedServices != nil {
		e["blocked_services"] = s.BlockedServices
	}
	if s.Upstreams != nil {
		e["upstreams"] = s.Upstreams
	}
	name, _ := e["name"].(string)
	return c.do(ctx, http.MethodPost, "/control/clients/update", map[string]any{"name": name, "data": e}, nil)
}

func defaultSafeSearch() map[string]any {
	m := map[string]any{"enabled": false}
	for _, k := range []string{"bing", "duckduckgo", "ecosia", "google", "pixabay", "yandex", "youtube"} {
		m[k] = true
	}
	return m
}

func (c *Client) Delete(ctx context.Context, id int64) error {
	e, err := c.findClient(ctx, id)
	if err != nil || e == nil {
		return err
	}
	return c.do(ctx, http.MethodPost, "/control/clients/delete", map[string]any{"name": e["name"]}, nil)
}

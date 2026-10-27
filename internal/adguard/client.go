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
	"time"
)

var ErrDisabled = errors.New("integrasi AdGuard belum diatur (UMBRA_ADGUARD_URL kosong)")

type Client struct {
	base string
	user string
	pass string
	http *http.Client
}

func New(base, user, pass string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"),
		user: user,
		pass: pass,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Enabled() bool { return c != nil && c.base != "" }

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("AdGuard tidak bisa dihubungi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("AdGuard %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
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

func (c *Client) SetProtection(ctx context.Context, enabled bool) error {
	return c.do(ctx, http.MethodPost, "/control/protection", map[string]any{"enabled": enabled}, nil)
}

// Stats dikembalikan apa adanya supaya frontend bisa memakai field yang dibutuhkan.
func (c *Client) Stats(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, "/control/stats", nil, &raw)
	return raw, err
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

// ClientName: penanda [wg#id] membuat panel bisa mengenali client buatannya sendiri.
func ClientName(name string, id int64) string {
	return fmt.Sprintf("%s [wg#%d]", name, id)
}

func Tag(id int64) string { return fmt.Sprintf("[wg#%d]", id) }

// Upsert membuat atau memperbarui client AdGuard untuk satu klien WireGuard.
func (c *Client) Upsert(ctx context.Context, id int64, name, ip string, adblock bool) error {
	existing, err := c.ListClients(ctx)
	if err != nil {
		return err
	}
	pc := PersistentClient{
		Name:                     ClientName(name, id),
		IDs:                      []string{ip},
		UseGlobalSettings:        false,
		FilteringEnabled:         adblock,
		SafebrowsingEnabled:      adblock,
		UseGlobalBlockedServices: true,
		Tags:                     []string{},
		Upstreams:                []string{},
	}
	for _, e := range existing {
		if strings.HasSuffix(e.Name, Tag(id)) {
			return c.do(ctx, http.MethodPost, "/control/clients/update",
				map[string]any{"name": e.Name, "data": pc}, nil)
		}
	}
	return c.do(ctx, http.MethodPost, "/control/clients/add", pc, nil)
}

func (c *Client) Delete(ctx context.Context, id int64) error {
	existing, err := c.ListClients(ctx)
	if err != nil {
		return err
	}
	for _, e := range existing {
		if strings.HasSuffix(e.Name, Tag(id)) {
			return c.do(ctx, http.MethodPost, "/control/clients/delete", map[string]any{"name": e.Name}, nil)
		}
	}
	return nil
}

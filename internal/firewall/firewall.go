// Package firewall manages the panel's own nftables table ("inet umbra").
// Because the table is separate, rules are never duplicated and never interfere
// with UFW/Docker rules. Every apply is atomic (a single nft transaction).
package firewall

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const Table = "umbra"

var ifaceRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,15}$`)

type Rules struct {
	WGInterface  string `json:"wg_interface"`
	OutInterface string `json:"out_interface"`
	Subnet       string `json:"subnet"` // e.g. 10.8.0.0/24
	MSSClamp     bool   `json:"mss_clamp"`
}

func (r Rules) Validate() error {
	if !ifaceRe.MatchString(r.WGInterface) {
		return fmt.Errorf("invalid WireGuard interface name %q", r.WGInterface)
	}
	if !ifaceRe.MatchString(r.OutInterface) {
		return fmt.Errorf("invalid outbound interface name %q", r.OutInterface)
	}
	if _, _, err := net.ParseCIDR(r.Subnet); err != nil {
		return fmt.Errorf("invalid subnet %q", r.Subnet)
	}
	return nil
}

// Render produces the nft script. The first line ensures the table exists, then it is deleted
// and recreated in the same transaction, so there is no gap without rules.
func Render(r Rules) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	_, n, _ := net.ParseCIDR(r.Subnet)
	var b strings.Builder
	fmt.Fprintf(&b, "table inet %s\ndelete table inet %s\n", Table, Table)
	fmt.Fprintf(&b, "table inet %s {\n", Table)
	b.WriteString("\tchain forward {\n\t\ttype filter hook forward priority filter; policy accept;\n")
	if r.MSSClamp {
		fmt.Fprintf(&b, "\t\toifname %q tcp flags & (syn|rst) == syn tcp option maxseg size set rt mtu\n", r.WGInterface)
		fmt.Fprintf(&b, "\t\tiifname %q tcp flags & (syn|rst) == syn tcp option maxseg size set rt mtu\n", r.WGInterface)
	}
	fmt.Fprintf(&b, "\t\tiifname %q accept\n", r.WGInterface)
	fmt.Fprintf(&b, "\t\toifname %q ct state established,related accept\n", r.WGInterface)
	b.WriteString("\t}\n")
	b.WriteString("\tchain postrouting {\n\t\ttype nat hook postrouting priority srcnat; policy accept;\n")
	fmt.Fprintf(&b, "\t\tip saddr %s oifname %q masquerade\n", n.String(), r.OutInterface)
	b.WriteString("\t}\n}\n")
	return b.String(), nil
}

// Manager applies rules with a trial period: if not confirmed within the
// time limit, the table is removed again (so a misconfiguration cannot lock out the server).
type Manager struct {
	fake bool

	mu      sync.Mutex
	timer   *time.Timer
	pending *Rules
	expires time.Time
}

func NewManager(fake bool) *Manager { return &Manager{fake: fake} }

type Pending struct {
	Rules   Rules     `json:"rules"`
	Expires time.Time `json:"expires"`
}

func (m *Manager) Pending() *Pending {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending == nil {
		return nil
	}
	return &Pending{Rules: *m.pending, Expires: m.expires}
}

// ApplyTrial applies the rules and sets a rollback timer. If not confirmed,
// the previous rules (prev) are reinstated; if prev is nil, the panel table is removed.
func (m *Manager) ApplyTrial(r Rules, wait time.Duration, prev *Rules) error {
	script, err := Render(r)
	if err != nil {
		return err
	}
	var prevScript string
	if prev != nil {
		if prevScript, err = Render(*prev); err != nil {
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.run(script); err != nil {
		return err
	}
	if m.timer != nil {
		m.timer.Stop()
	}
	m.pending = &r
	m.expires = time.Now().Add(wait)
	m.timer = time.AfterFunc(wait, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.pending == nil {
			return
		}
		slog.Warn("firewall not confirmed, reverting to previous rules")
		m.pending = nil
		var err error
		if prevScript != "" {
			err = m.run(prevScript)
		} else {
			err = m.remove()
		}
		if err != nil {
			slog.Error("firewall rollback failed", "err", err)
		}
	})
	return nil
}

// Confirm stops the rollback timer and returns the confirmed rules.
func (m *Manager) Confirm() (Rules, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending == nil {
		return Rules{}, errors.New("no firewall change is awaiting confirmation")
	}
	if m.timer != nil {
		m.timer.Stop()
	}
	r := *m.pending
	m.pending = nil
	return r, nil
}

// ApplyNow applies without a trial period (used at boot for already-confirmed rules).
func (m *Manager) ApplyNow(r Rules) error {
	script, err := Render(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.run(script)
}

// Remove deletes the panel's table and cancels any trial in progress.
func (m *Manager) Remove() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.timer != nil {
		m.timer.Stop()
	}
	m.pending = nil
	return m.remove()
}

func (m *Manager) remove() error {
	return m.run(fmt.Sprintf("table inet %s\ndelete table inet %s\n", Table, Table))
}

func (m *Manager) run(script string) error {
	if m.fake {
		slog.Info("fake firewall: nft -f -", "script", script)
		return nil
	}
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nft failed: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

package wg

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"syscall"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

// ErrKernelUnsupported marks a kernel that cannot create WireGuard links.
var ErrKernelUnsupported = errors.New("kernel does not support WireGuard")

func kernelSupportError(err error) bool {
	return errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EPROTONOSUPPORT) ||
		errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENOSYS)
}

// New selects a backend when the interface is first set up.
func New(mode string) (Backend, error) {
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "kernel" && mode != "userspace" {
		return nil, fmt.Errorf("UMBRA_WG_BACKEND must be auto, kernel, or userspace: %q", mode)
	}
	return &managed{mode: mode, kernel: NewKernel, userspace: newUserspace}, nil
}

type managed struct {
	mu                sync.Mutex
	mode              string
	kernel, userspace func() (Backend, error)
	active            Backend
	closed            bool
}

func (m *managed) Name() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return "pending"
	}
	return m.active.Name()
}

func (m *managed) EnsureInterface(cfg store.ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureLocked(cfg)
}

func (m *managed) ensureLocked(cfg store.ServerConfig) error {
	if m.closed {
		return errors.New("WireGuard backend is already closed")
	}
	if m.active != nil {
		return m.active.EnsureInterface(cfg)
	}
	factory := m.kernel
	if m.mode == "userspace" {
		factory = m.userspace
	}
	b, err := factory()
	if err != nil {
		return err
	}
	err = b.EnsureInterface(cfg)
	if err != nil {
		if closeErr := b.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		if m.mode != "auto" || !errors.Is(err, ErrKernelUnsupported) {
			return err
		}
		slog.Warn("WireGuard kernel unavailable, falling back to wireguard-go userspace", "err", err)
		b, err = m.userspace()
		if err != nil {
			return err
		}
		if err = b.EnsureInterface(cfg); err != nil {
			return errors.Join(err, b.Close())
		}
	}
	m.active = b
	slog.Info("WireGuard backend active", "backend", b.Name(), "interface", cfg.Interface)
	return nil
}

// Sync sets up the interface first if no backend is active yet (e.g. bootstrap failed
// temporarily), so device changes do not keep failing until the service is restarted.
func (m *managed) Sync(cfg store.ServerConfig, peers []Peer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("WireGuard backend is already closed")
	}
	if m.active == nil {
		if err := m.ensureLocked(cfg); err != nil {
			return err
		}
	}
	return m.active.Sync(cfg, peers)
}

func (m *managed) Stats(iface string) ([]PeerStat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.active == nil {
		return nil, errors.New("WireGuard backend not ready")
	}
	return m.active.Stats(iface)
}

func (m *managed) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	if m.active != nil {
		return m.active.Close()
	}
	return nil
}

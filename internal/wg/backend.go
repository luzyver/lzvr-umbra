package wg

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"syscall"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

// ErrKernelUnsupported menandai kernel yang tidak bisa membuat link WireGuard.
var ErrKernelUnsupported = errors.New("kernel tidak mendukung WireGuard")

func kernelSupportError(err error) bool {
	return errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EPROTONOSUPPORT) ||
		errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENOSYS)
}

// New memilih backend saat interface pertama kali disiapkan.
func New(mode string) (Backend, error) {
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "kernel" && mode != "userspace" {
		return nil, fmt.Errorf("UMBRA_WG_BACKEND harus auto, kernel, atau userspace: %q", mode)
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
		return errors.New("backend WireGuard sudah ditutup")
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
		slog.Warn("kernel WireGuard tidak tersedia, beralih ke wireguard-go userspace", "err", err)
		b, err = m.userspace()
		if err != nil {
			return err
		}
		if err = b.EnsureInterface(cfg); err != nil {
			return errors.Join(err, b.Close())
		}
	}
	m.active = b
	slog.Info("backend WireGuard aktif", "backend", b.Name(), "interface", cfg.Interface)
	return nil
}

// Sync menyiapkan interface dulu bila belum ada backend aktif (mis. bootstrap gagal
// sementara), supaya perubahan perangkat tidak gagal terus sampai service di-restart.
func (m *managed) Sync(cfg store.ServerConfig, peers []Peer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("backend WireGuard sudah ditutup")
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
		return nil, errors.New("backend WireGuard belum siap")
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

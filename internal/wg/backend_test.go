package wg

import (
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

// probeBackend mengganti operasi jaringan yang membutuhkan root.
type probeBackend struct {
	Fake
	name            string
	err             error
	ensures, closes int
}

func (b *probeBackend) Name() string                             { return b.name }
func (b *probeBackend) EnsureInterface(store.ServerConfig) error { b.ensures++; return b.err }
func (b *probeBackend) Close() error                             { b.closes++; return nil }

func TestBackendSelection(t *testing.T) {
	for _, tt := range []struct {
		mode                        string
		kernelErr                   error
		want                        string
		wantErr                     bool
		kernelCalls, userspaceCalls int
	}{
		{"", nil, "kernel", false, 2, 0},
		{"auto", nil, "kernel", false, 2, 0},
		{"auto", fmt.Errorf("buat link: %w", ErrKernelUnsupported), "userspace", false, 1, 2},
		{"auto", syscall.EPERM, "pending", true, 1, 0},
		{"auto", syscall.EINVAL, "pending", true, 1, 0},
		{"kernel", ErrKernelUnsupported, "pending", true, 1, 0},
		{"userspace", nil, "userspace", false, 0, 2},
	} {
		t.Run(fmt.Sprintf("%s/%v", tt.mode, tt.kernelErr), func(t *testing.T) {
			k := &probeBackend{name: "kernel", err: tt.kernelErr}
			u := &probeBackend{name: "userspace"}
			backend, err := New(tt.mode)
			if err != nil {
				t.Fatal(err)
			}
			b := backend.(*managed)
			b.kernel = func() (Backend, error) { return k, nil }
			b.userspace = func() (Backend, error) { return u, nil }
			err = b.EnsureInterface(store.ServerConfig{})
			if (err != nil) != tt.wantErr {
				t.Fatalf("EnsureInterface: %v", err)
			}
			if !tt.wantErr {
				if err := b.EnsureInterface(store.ServerConfig{}); err != nil {
					t.Fatal(err)
				}
			}
			if b.Name() != tt.want || k.ensures != tt.kernelCalls || u.ensures != tt.userspaceCalls {
				t.Fatalf("backend=%s, kernel=%d, userspace=%d", b.Name(), k.ensures, u.ensures)
			}
			if err := b.Close(); err != nil {
				t.Fatal(err)
			}
			if err := b.Close(); err != nil {
				t.Fatal(err)
			}
			if !tt.wantErr && ((tt.want == "kernel" && k.closes != 1) || (tt.want == "userspace" && u.closes != 1)) {
				t.Fatal("backend aktif harus ditutup sekali")
			}
			if err := b.EnsureInterface(store.ServerConfig{}); err == nil {
				t.Fatal("backend tertutup bisa dibuka lagi")
			}
		})
	}
}

func TestNewBackendRejectsInvalidMode(t *testing.T) {
	if _, err := New("typo"); err == nil {
		t.Fatal("mode tidak valid diterima")
	}
}

func TestKernelSupportError(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want bool
	}{
		{syscall.EOPNOTSUPP, true}, {syscall.EPROTONOSUPPORT, true},
		{syscall.ENODEV, true}, {syscall.ENOSYS, true},
		{syscall.EPERM, false}, {syscall.EACCES, false}, {syscall.EINVAL, false},
		{syscall.EEXIST, false}, {errors.New("operation not supported"), false}, {nil, false},
	} {
		if got := kernelSupportError(fmt.Errorf("buat link: %w", tt.err)); got != tt.want {
			t.Errorf("%v: got %v", tt.err, got)
		}
	}
}

func TestSyncEnsuresInterfaceWhenPending(t *testing.T) {
	k := &probeBackend{name: "kernel"}
	backend, err := New("kernel")
	if err != nil {
		t.Fatal(err)
	}
	b := backend.(*managed)
	b.kernel = func() (Backend, error) { return k, nil }
	if err := b.Sync(store.ServerConfig{}, nil); err != nil {
		t.Fatal(err)
	}
	if b.Name() != "kernel" || k.ensures != 1 {
		t.Fatalf("backend=%s, ensures=%d", b.Name(), k.ensures)
	}
}

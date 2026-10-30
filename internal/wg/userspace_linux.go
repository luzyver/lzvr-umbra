//go:build linux

package wg

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

// userspace mewarisi Sync/Stats wgctrl; perangkat diakses melalui socket UAPI.
type userspace struct {
	Kernel
	mu     sync.Mutex
	dev    *device.Device
	uapi   *uapiServer
	iface  string
	closed bool
}

func newUserspace() (Backend, error) { return &userspace{}, nil }
func (*userspace) Name() string      { return "userspace" }

var userspaceIfaceRE = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,15}$`)

func (u *userspace) EnsureInterface(cfg store.ServerConfig) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return errors.New("backend userspace sudah ditutup")
	}
	if !userspaceIfaceRE.MatchString(cfg.Interface) || cfg.Interface == "." || cfg.Interface == ".." {
		return errors.New("nama interface userspace tidak valid")
	}
	if u.dev != nil {
		if cfg.Interface != u.iface {
			return errors.New("nama interface userspace tidak bisa diganti selama proses berjalan")
		}
		link, err := netlink.LinkByName(u.iface)
		if err != nil {
			return err
		}
		return configureLink(link, cfg)
	}
	// Jangan mengambil alih interface milik kernel atau proses lain.
	if _, err := netlink.LinkByName(cfg.Interface); err == nil {
		return fmt.Errorf("interface %s sudah ada; hentikan pemiliknya sebelum memakai userspace", cfg.Interface)
	} else {
		var notFound netlink.LinkNotFoundError
		if !errors.As(err, &notFound) {
			return err
		}
	}
	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = device.DefaultMTU
	}
	tdev, err := tun.CreateTUN(cfg.Interface, mtu)
	if err != nil {
		return fmt.Errorf("buat TUN %s (periksa /dev/net/tun dan CAP_NET_ADMIN): %w", cfg.Interface, err)
	}
	dev := device.NewDevice(tdev, conn.NewDefaultBind(), &device.Logger{
		Errorf:   func(format string, args ...any) { slog.Error("wireguard-go: " + fmt.Sprintf(format, args...)) },
		Verbosef: func(string, ...any) {},
	})
	ready := false
	defer func() {
		if !ready {
			dev.Close()
		}
	}()
	file, err := ipc.UAPIOpen(cfg.Interface)
	if err != nil {
		return fmt.Errorf("buka UAPI: %w", err)
	}
	listener, listenErr := ipc.UAPIListen(cfg.Interface, file)
	closeErr := file.Close() // UAPIListen sudah menduplikasi deskriptor.
	if listenErr != nil {
		return fmt.Errorf("dengarkan UAPI: %w", listenErr)
	}
	if closeErr != nil {
		_ = listener.Close()
		return closeErr
	}
	server := startUAPI(listener, dev.IpcHandle)
	link, err := netlink.LinkByName(cfg.Interface)
	if err == nil {
		err = configureLink(link, cfg)
	}
	if err != nil {
		return errors.Join(err, server.Close())
	}
	u.dev, u.uapi, u.iface = dev, server, cfg.Interface
	ready = true
	slog.Info("wireguard-go userspace siap", "uapi", "/var/run/wireguard/"+cfg.Interface+".sock")
	return nil
}

func (u *userspace) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return nil
	}
	u.closed = true
	var err error
	if u.uapi != nil {
		err = u.uapi.Close()
	}
	if u.dev != nil {
		u.dev.Close()
	}
	return err
}

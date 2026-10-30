//go:build linux

package wg

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/luzyver/lzvr-umbra/internal/store"
)

type Kernel struct{}

func NewKernel() (Backend, error) { return Kernel{}, nil }

func (Kernel) Name() string { return "kernel" }
func (Kernel) Close() error { return nil }

func (Kernel) EnsureInterface(cfg store.ServerConfig) error {
	link, err := netlink.LinkByName(cfg.Interface)
	if err != nil {
		var notFound netlink.LinkNotFoundError
		if !errors.As(err, &notFound) {
			return fmt.Errorf("cek interface %s: %w", cfg.Interface, err)
		}
		la := netlink.NewLinkAttrs()
		la.Name = cfg.Interface
		if err := netlink.LinkAdd(&netlink.GenericLink{LinkAttrs: la, LinkType: "wireguard"}); err != nil {
			if kernelSupportError(err) {
				return fmt.Errorf("buat interface %s: %w: %w", cfg.Interface, ErrKernelUnsupported, err)
			}
			return fmt.Errorf("buat interface %s (modul wireguard sudah dimuat?): %w", cfg.Interface, err)
		}
		if link, err = netlink.LinkByName(cfg.Interface); err != nil {
			return err
		}
	}
	if link.Type() != "wireguard" {
		return fmt.Errorf("interface %s sudah ada dengan tipe %s, bukan WireGuard kernel", cfg.Interface, link.Type())
	}
	return configureLink(link, cfg)
}

// configureLink dipakai untuk link kernel maupun TUN userspace.
func configureLink(link netlink.Link, cfg store.ServerConfig) error {
	if cfg.MTU > 0 {
		if err := netlink.LinkSetMTU(link, cfg.MTU); err != nil {
			return fmt.Errorf("set MTU: %w", err)
		}
	}
	addr, err := netlink.ParseAddr(cfg.Address)
	if err != nil {
		return fmt.Errorf("alamat server %q: %w", cfg.Address, err)
	}
	if err := netlink.AddrReplace(link, addr); err != nil {
		return fmt.Errorf("pasang alamat: %w", err)
	}
	return netlink.LinkSetUp(link)
}

func (Kernel) Sync(cfg store.ServerConfig, peers []Peer) error {
	c, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer c.Close()

	priv, err := wgtypes.ParseKey(cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("private key server: %w", err)
	}
	port := cfg.ListenPort
	pcs := make([]wgtypes.PeerConfig, 0, len(peers))
	for _, p := range peers {
		pub, err := wgtypes.ParseKey(p.PublicKey)
		if err != nil {
			return fmt.Errorf("public key %s: %w", p.PublicKey, err)
		}
		ip := net.ParseIP(p.Address).To4()
		if ip == nil {
			return fmt.Errorf("alamat klien %q tidak valid", p.Address)
		}
		pc := wgtypes.PeerConfig{
			PublicKey:         pub,
			ReplaceAllowedIPs: true,
			AllowedIPs:        []net.IPNet{{IP: ip, Mask: net.CIDRMask(32, 32)}},
		}
		if p.PresharedKey != "" {
			psk, err := wgtypes.ParseKey(p.PresharedKey)
			if err != nil {
				return err
			}
			pc.PresharedKey = &psk
		}
		if p.Keepalive > 0 {
			d := time.Duration(p.Keepalive) * time.Second
			pc.PersistentKeepaliveInterval = &d
		}
		pcs = append(pcs, pc)
	}
	// ReplacePeers: peer yang tidak ada di DB ikut terhapus dari kernel
	return c.ConfigureDevice(cfg.Interface, wgtypes.Config{
		PrivateKey:   &priv,
		ListenPort:   &port,
		ReplacePeers: true,
		Peers:        pcs,
	})
}

func (Kernel) Stats(iface string) ([]PeerStat, error) {
	c, err := wgctrl.New()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	dev, err := c.Device(iface)
	if err != nil {
		return nil, err
	}
	out := make([]PeerStat, 0, len(dev.Peers))
	for _, p := range dev.Peers {
		s := PeerStat{
			PublicKey:     p.PublicKey.String(),
			LastHandshake: p.LastHandshakeTime,
			RxBytes:       p.ReceiveBytes,
			TxBytes:       p.TransmitBytes,
		}
		if p.Endpoint != nil {
			s.Endpoint = p.Endpoint.String()
		}
		out = append(out, s)
	}
	return out, nil
}

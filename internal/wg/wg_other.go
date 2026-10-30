//go:build !linux

package wg

import "errors"

func NewKernel() (Backend, error) {
	return nil, errors.New("WireGuard kernel hanya tersedia di Linux; jalankan dengan UMBRA_DEV=1 untuk mode tiruan")
}

func newUserspace() (Backend, error) {
	return nil, errors.New("WireGuard userspace hanya tersedia di Linux; jalankan dengan UMBRA_DEV=1 untuk mode tiruan")
}

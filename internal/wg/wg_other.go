//go:build !linux

package wg

import "errors"

func NewKernel() (Backend, error) {
	return nil, errors.New("kernel WireGuard is only available on Linux; run with UMBRA_DEV=1 for fake mode")
}

func newUserspace() (Backend, error) {
	return nil, errors.New("userspace WireGuard is only available on Linux; run with UMBRA_DEV=1 for fake mode")
}

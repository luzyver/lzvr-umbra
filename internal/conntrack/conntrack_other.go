//go:build !linux

package conntrack

func list() ([]Conn, error) { return nil, ErrUnsupported }

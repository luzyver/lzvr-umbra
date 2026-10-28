//go:build linux

package conntrack

import (
	"time"

	"github.com/vishvananda/netlink"
)

func list() ([]Conn, error) {
	var out []Conn
	for _, fam := range []netlink.InetFamily{netlink.FAMILY_V4, netlink.FAMILY_V6} {
		flows, err := netlink.ConntrackTableList(netlink.ConntrackTable, fam)
		if err != nil {
			return nil, err
		}
		for _, f := range flows {
			c := Conn{
				Proto:    protoName(f.Forward.Protocol),
				SrcIP:    f.Forward.SrcIP.String(),
				SrcPort:  f.Forward.SrcPort,
				DstIP:    f.Forward.DstIP.String(),
				DstPort:  f.Forward.DstPort,
				Sent:     f.Forward.Bytes,
				Received: f.Reverse.Bytes,
				Packets:  f.Forward.Packets + f.Reverse.Packets,
				Expires:  int(f.TimeOut),
			}
			if tcp, ok := f.ProtoInfo.(*netlink.ProtoInfoTCP); ok {
				c.State = tcpState(tcp.State)
			}
			if f.TimeStart > 0 {
				t := time.Unix(0, int64(f.TimeStart))
				c.Started = &t
			}
			out = append(out, c)
		}
	}
	return out, nil
}

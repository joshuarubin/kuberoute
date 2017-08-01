package main

import (
	"net"
	"syscall"

	"golang.org/x/net/route"
)

// this was largely copied from go/src/net/interface_bsd.go
func p2pDest(ifindex int, msgs []route.Message) net.IP {
	for _, m := range msgs {
		switch m := m.(type) {
		case *route.InterfaceAddrMessage:
			if ifindex != 0 && ifindex != m.Index {
				continue
			}

			// RTAX_BRD is the index of the point-to-point address

			var ip net.IP
			switch sa := m.Addrs[syscall.RTAX_BRD].(type) {
			case *route.Inet4Addr:
				ip = net.IPv4(sa.IP[0], sa.IP[1], sa.IP[2], sa.IP[3])
			case *route.Inet6Addr:
				ip = make(net.IP, net.IPv6len)
				copy(ip, sa.IP[:])
			}

			var foundNonZero bool
			for _, b := range ip {
				if b != 0 {
					foundNonZero = true
					break
				}
			}

			if !foundNonZero {
				continue
			}

			// found it
			return ip
		}
	}

	return nil
}

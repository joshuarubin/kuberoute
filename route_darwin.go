package main

import (
	"net"
	"os"
	"syscall"

	"golang.org/x/net/route"
)

func routeMessage(i int, ip, gw net.IP) route.RouteMessage {
	m := route.RouteMessage{
		Type: syscall.RTM_ADD,
		ID:   uintptr(os.Getpid()),
		Seq:  i + 1,
	}

	if ip.To4() != nil {
		m.Addrs = []route.Addr{
			syscall.RTAX_DST:     &route.Inet4Addr{IP: ip4(ip)},
			syscall.RTAX_GATEWAY: &route.Inet4Addr{IP: ip4(gw)},
		}
	} else if ip.To16() != nil {
		m.Addrs = []route.Addr{
			syscall.RTAX_DST:     &route.Inet6Addr{IP: ip6(ip)},
			syscall.RTAX_GATEWAY: &route.Inet6Addr{IP: ip6(gw)},
		}
	}

	return m
}

// copied from go/src/net/interface_darwin.go
func interfaceMessages(ifindex int) ([]route.Message, error) {
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, syscall.NET_RT_IFLIST, ifindex)
	if err != nil {
		return nil, err
	}
	return route.ParseRIB(syscall.NET_RT_IFLIST, rib)
}

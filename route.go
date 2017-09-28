package main // import "jrubin.io/kuberoute"

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func kubeConfig() (*restclient.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	var configOverrides clientcmd.ConfigOverrides
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &configOverrides)
	return kubeConfig.ClientConfig()
}

func hostAddrs(hosts ...string) ([]string, error) {
	addrs := map[string]struct{}{}

	for _, h := range hosts {
		host := h
		if !strings.Contains(host, "://") {
			host = "http://" + host
		}

		u, err := url.Parse(host)
		if err != nil {
			return nil, err
		}

		as, err := net.LookupHost(u.Host)
		if err != nil {
			return nil, fmt.Errorf("error looking up %s: %s", h, err)
		}

		for _, a := range as {
			addrs[a] = struct{}{}
		}
	}

	var ret []string
	for h := range addrs {
		ret = append(ret, h)
	}
	return ret, nil
}

func ip4(ip net.IP) (ret [4]byte) {
	copy(ret[:], ip.To4())
	return
}

func ip6(ip net.IP) (ret [16]byte) {
	copy(ret[:], ip.To16())
	return
}

func addRoutes(gw net.IP, addrs []string) error {
	sock, err := syscall.Socket(syscall.AF_ROUTE, syscall.SOCK_RAW, syscall.AF_UNSPEC)
	if err != nil {
		return err
	}
	defer func() { _ = syscall.Close(sock) }() // #nosec

	for i, addr := range addrs {
		ip := net.ParseIP(addr)
		fmt.Printf("routing %s through %s\n", ip, gw)

		m := routeMessage(i, ip, gw)

		data, err := m.Marshal()
		if err != nil {
			return err
		}

		if _, err = syscall.Write(sock, data); err != nil {
			fmt.Fprintf(os.Stderr, "error adding route %s: %s\n", addr, err) // #nosec
			continue
		}
	}

	return nil
}

func getGW() (net.IP, error) {
	// get the list of all network interfaces
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifs {
		const flags = net.FlagUp | net.FlagPointToPoint

		// only consider interfaces that are up and point-to-point
		if iface.Flags&flags != flags {
			continue
		}

		iaddrs, aerr := iface.Addrs()
		if aerr != nil {
			return nil, aerr
		}

		var ips []net.IP

		for _, addr := range iaddrs {
			ip, _, perr := net.ParseCIDR(addr.String())
			if perr != nil {
				return nil, perr
			}

			// ignore link local addresses
			if ip.IsLinkLocalUnicast() {
				continue
			}

			ips = append(ips, ip)
		}

		// ignore interfaces with no addresses
		if len(ips) == 0 {
			continue
		}

		// have to dig deep to be able to get the destination address of a
		// point-to-point interfae
		msgs, err := interfaceMessages(iface.Index)
		if err != nil {
			return nil, err
		}

		if gw := p2pDest(iface.Index, msgs); gw != nil {
			return gw, nil
		}
	}

	return nil, errors.New("could not determine gateway ip")
}

func main() {
	config, err := kubeConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting kubeconfig: %s\n", err) // #nosec
		return
	}

	fmt.Printf("kubernetes host: %s\n", config.Host)

	hosts := make([]string, len(os.Args)-1)
	copy(hosts, os.Args[1:])
	hosts = append(hosts, config.Host)

	addrs, err := hostAddrs(hosts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err) // #nosec
		return
	}

	gw, err := getGW()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting gateway: %s\n", err) // #nosec
		return
	}

	if err = addRoutes(gw, addrs); err != nil {
		fmt.Fprintf(os.Stderr, "error adding routes: %s\n", err) // #nosec
	}
}

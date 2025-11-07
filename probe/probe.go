package probe

import (
	"log"
	"net"
	"time"
)

// IpsFromCIDR: returns []string of IPs (excludes network and broadcast for v4)
func IpsFromCIDR(cidr string) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	var ips []string
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); nextIP(ip) {
		ips = append(ips, ip.String())
	}
	// remove network and broadcast if IPv4 and length>2
	if len(ips) > 2 && ipnet.IP.To4() != nil {
		return ips[1 : len(ips)-1], nil
	}
	return ips, nil
}

func nextIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}

func DialAddress(addr string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("connected to address %s successfully", addr)
	return nil
}

package probe

import (
	"log"
	"net"
	"strings"
	"time"
)

// IpsFromCIDR: returns []string of IPs (excludes network and broadcast for v4)
func IpsFromCidrs(cidrs string) ([]string, error) {
	cidrsList := strings.Split(cidrs, ",")
	var allIps []string
	for _, cidr := range cidrsList {
		ips := make([]string, 0)
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}

		for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); nextIP(ip) {
			ips = append(ips, ip.String())
		}
		// remove network and broadcast if IPv4 and length>2
		if len(ips) > 2 && ipnet.IP.To4() != nil {
			allIps = append(allIps, ips[1:len(ips)-1]...)
		} else {
			allIps = append(allIps, ips...)
		}
	}
	return allIps, nil
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

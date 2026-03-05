package main

import (
	"fmt"
	"net"
)

// defaultVIPFromCIDR calculates a default VIP address from a CIDR at the given offset.
func defaultVIPFromCIDR(cidr string, offset int) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %w", err)
	}

	networkIP := ipNet.IP.Mask(ipNet.Mask)
	networkInt := ipToInt(networkIP)
	vipInt := networkInt + uint32(offset)
	vip := intToIP(vipInt)

	return vip.String(), nil
}

func ipInCIDR(ipStr, cidr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	return ipNet.Contains(ip)
}

func ipToInt(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func intToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

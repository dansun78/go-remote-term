// Package network provides network-related utilities for the go-remote-term application.
package network

import (
	"fmt"
	"net"
)

// GetLocalIPAddresses returns all non-loopback IP addresses of the host.
// This is useful when binding to 0.0.0.0 to determine all possible
// IP addresses that clients might use to connect to the server.
func GetLocalIPAddresses() ([]string, error) {
	var ips []string

	// Get all network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	// Iterate through all interfaces
	for _, iface := range interfaces {
		// Skip loopback, down, and unassigned interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Get addresses for this interface
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		// Extract IP addresses
		for _, addr := range addrs {
			var ip net.IP

			// Extract IP from address
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip nil IPs and loopback addresses
			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Only use IPv4 and IPv6 addresses that aren't link-local
			if ip.To4() != nil || (!ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()) {
				ips = append(ips, ip.String())
			}
		}
	}

	return ips, nil
}

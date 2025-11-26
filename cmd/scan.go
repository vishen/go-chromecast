// Copyright © 2024 Martin Holst Swende
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/seancfoley/ipaddress-go/ipaddr"
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/application"
)

// scanCmd triggers a scan
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for chromecast devices",
Run: func(cmd *cobra.Command, args []string) {
	subnetsFlag, _ := cmd.Flags().GetString("subnets")
	broadSearch, _ := cmd.Flags().GetBool("broad-search")
	ports, _ := cmd.Flags().GetIntSlice("ports")
	ifaceName, _ := cmd.Flags().GetString("iface")
	var subnets []string

	if subnetsFlag != "" {
		if subnetsFlag == "*" {
			// Scan all detected subnets on all active interfaces, skipping virtual/Tailscale interfaces
			interfaces, err := net.Interfaces()
			if err != nil {
				exit("could not list interfaces: %v", err)
			}
			for _, iface := range interfaces {
				if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
					continue
				}
				// Skip Tailscale and common virtual interfaces
				name := strings.ToLower(iface.Name)
				if strings.HasPrefix(name, "tailscale") || strings.HasPrefix(name, "ts") || strings.HasPrefix(name, "utun") || strings.HasPrefix(name, "tun") || strings.HasPrefix(name, "tap") || strings.HasPrefix(name, "vmnet") || strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "vbox") || strings.HasPrefix(name, "zt") || strings.HasPrefix(name, "wg") {
					continue
				}
				addrs, err := iface.Addrs()
				if err != nil {
					continue
				}
				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							network := ipnet.IP.Mask(net.CIDRMask(24, 32))
							subnet := fmt.Sprintf("%s/24", network.String())
							subnets = append(subnets, subnet)
						}
					}
				}
			}
			if len(subnets) == 0 {
				exit("could not detect any subnets for broad search")
			}
		} else {
			// Parse comma-separated list
			for _, s := range splitAndTrim(subnetsFlag, ",") {
				if s != "" {
					subnets = append(subnets, s)
				}
			}
		}
	} else if broadSearch {
		// Scan all detected subnets on all active interfaces, skipping virtual/Tailscale interfaces
		interfaces, err := net.Interfaces()
		if err != nil {
			exit("could not list interfaces: %v", err)
		}
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			// Skip Tailscale and common virtual interfaces
			name := strings.ToLower(iface.Name)
			if strings.HasPrefix(name, "tailscale") || strings.HasPrefix(name, "ts") || strings.HasPrefix(name, "utun") || strings.HasPrefix(name, "tun") || strings.HasPrefix(name, "tap") || strings.HasPrefix(name, "vmnet") || strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "vbox") || strings.HasPrefix(name, "zt") || strings.HasPrefix(name, "wg") {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						network := ipnet.IP.Mask(net.CIDRMask(24, 32))
						subnet := fmt.Sprintf("%s/24", network.String())
						subnets = append(subnets, subnet)
					}
				}
			}
		}
		if len(subnets) == 0 {
			exit("could not detect any subnets for broad search")
		}
	} else {
		cidrAddr, _ := cmd.Flags().GetString("cidr")
		// If no CIDR was explicitly provided, try to auto-detect the local subnet
		if cidrAddr == "192.168.50.0/24" {
			if detectedCIDR, err := detectLocalSubnet(ifaceName); err == nil {
				cidrAddr = detectedCIDR
			}
		}
		subnets = []string{cidrAddr}
	}

	totalCount := 0
	start := time.Now()
	for _, cidrAddr := range subnets {
		outputInfo("Scanning subnet %s...\n", cidrAddr)
		var (
			wg     sync.WaitGroup
			ipCh   = make(chan *ipaddr.IPAddress)
			logged = time.Unix(0, 0)
			count  int
		)
		ipRange, err := ipaddr.NewIPAddressString(cidrAddr).ToSequentialRange()
		if err != nil {
			outputError("could not parse cidr address expression: %v", err)
			continue
		}
		go func() {
			it := ipRange.Iterator()
			for it.HasNext() {
				ip := it.Next()
				if time.Since(logged) > 8*time.Second {
					outputInfo("Scanning...  scanned %d, current %v\n", count, ip.String())
					logged = time.Now()
				}
				ipCh <- ip
				count++
			}
			close(ipCh)
		}()
		for i := 0; i < 64; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				dialer := &net.Dialer{
					Timeout: 400 * time.Millisecond,
				}
				for ip := range ipCh {
					for _, port := range ports {
						conn, err := dialer.Dial("tcp", fmt.Sprintf("%v:%d", ip, port))
						if err != nil {
							continue
						}
						conn.Close()
						if info, err := application.GetInfo(ip.String()); err != nil {
							outputInfo("  - Device at %v:%d errored during discovery: %v", ip, port, err)
						} else {
							outputInfo("  - '%v' at %v:%d\n", info.Name, ip, port)
						}
					}
				}
			}()
		}
		wg.Wait()
		outputInfo("Scanned %d uris in %v for subnet %s\n", count, time.Since(start), cidrAddr)
	}
	outputInfo("Total scanned %d uris in %v\n", totalCount, time.Since(start))
	},
}

// splitAndTrim splits a string by sep and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	var out []string
	for _, part := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// detectLocalSubnet attempts to detect the local subnet based on the network interface
func detectLocalSubnet(ifaceName string) (string, error) {
	var iface *net.Interface
	var err error

	if ifaceName != "" {
		iface, err = net.InterfaceByName(ifaceName)
		if err != nil {
			return "", err
		}
	}

	if iface != nil {
		// Use the specified interface
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					// Return the network address with /24 subnet
					network := ipnet.IP.Mask(net.CIDRMask(24, 32))
					return fmt.Sprintf("%s/24", network.String()), nil
				}
			}
		}
	} else {
		// Try to find the default route interface
		interfaces, err := net.Interfaces()
		if err != nil {
			return "", err
		}

		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}

			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						// Return the network address with /24 subnet
						network := ipnet.IP.Mask(net.CIDRMask(24, 32))
						return fmt.Sprintf("%s/24", network.String()), nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not detect local subnet")
}

func init() {
	// Common Chromecast ports: 8009 (main), 8008, 8443
	// Common group ports are typically in the 32000+ range
	defaultPorts := []int{8009, 8008, 8443}
	// Add some common group port ranges
	for i := 32000; i <= 32010; i++ {
		defaultPorts = append(defaultPorts, i)
	}
	// Add the specific port we've seen (32236)
	defaultPorts = append(defaultPorts, 32236)

	scanCmd.Flags().String("cidr", "192.168.50.0/24", "cidr expression of subnet to scan")
	scanCmd.Flags().IntSlice("ports", defaultPorts, "ports to scan for (includes Chromecast devices and groups)")
	scanCmd.Flags().String("iface", "", "network interface to use for detecting local subnet")
	scanCmd.Flags().String("subnets", "", "Comma-separated list of subnets to scan (e.g. 192.168.4.0/24,192.168.3.0/24), or * for all detected subnets. Overrides --cidr and --broad-search if set.")
	scanCmd.Flags().BoolP("broad-search", "b", false, "(No-op) For consistency: scan always performs a comprehensive search")
	rootCmd.AddCommand(scanCmd)
}

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
		cidrAddr, _ := cmd.Flags().GetString("cidr")
		if cidrAddr == "" {
			ifaceName, _ := cmd.Flags().GetString("iface")
			var err error
			if cidrAddr, err = localCIDR(ifaceName); err != nil {
				exit("unable to work out the subnet to scan, specify one with --cidr: %v", err)
			}
			outputInfo("Scanning the local subnet %s, use --cidr to scan a different one\n", cidrAddr)
		}
		var (
			port, _      = cmd.Flags().GetInt("port")
			wg           sync.WaitGroup
			ipCh         = make(chan *ipaddr.IPAddress)
			logged       = time.Unix(0, 0)
			start        = time.Now()
			count        int
			ipRange, err = ipaddr.NewIPAddressString(cidrAddr).ToSequentialRange()
		)
		if err != nil {
			exit("could not parse cidr address expression: %v", err)
		}
		// Use one goroutine to send URIs over a channel
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
		// Use a bunch of goroutines to do connect-attempts.
		for i := 0; i < 64; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				dialer := &net.Dialer{
					Timeout: 400 * time.Millisecond,
				}
				for ip := range ipCh {
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
			}()
		}
		wg.Wait()
		outputInfo("Scanned %d uris in %v\n", count, time.Since(start))
	},
}

// localCIDR returns the cidr expression of the local IPv4 subnet to scan,
// looking only at the named network interface if one is given.
func localCIDR(ifaceName string) (string, error) {
	var ifaces []net.Interface
	if ifaceName != "" {
		iface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			return "", fmt.Errorf("unable to find interface %q: %w", ifaceName, err)
		}
		ifaces = []net.Interface{*iface}
	} else {
		var err error
		if ifaces, err = net.Interfaces(); err != nil {
			return "", err
		}
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if cidr := scanCIDR(ipnet); cidr != "" {
					return cidr, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no network interface with a private IPv4 address found")
}

// scanCIDR returns the cidr expression to scan for the address of a network
// interface, or an empty string if it isn't a private IPv4 address. Subnets
// bigger than a /24 are narrowed down to the /24 the address is in, as
// scanning them would take too long.
func scanCIDR(ipnet *net.IPNet) string {
	ip := ipnet.IP.To4()
	if ip == nil || !ip.IsPrivate() {
		return ""
	}
	ones, _ := ipnet.Mask.Size()
	if ones < 24 {
		ones = 24
	}
	network := ip.Mask(net.CIDRMask(ones, 32))
	return fmt.Sprintf("%s/%d", network, ones)
}

func init() {
	scanCmd.Flags().String("cidr", "", "cidr expression of subnet to scan (default: the local subnet)")
	scanCmd.Flags().Int("port", 8009, "port to scan for")
	rootCmd.AddCommand(scanCmd)
}

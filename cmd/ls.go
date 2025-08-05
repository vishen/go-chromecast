// Copyright © 2018 Jonathan Pentecost <pentecostjonathan@gmail.com>
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
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/seancfoley/ipaddress-go/ipaddr"
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List devices",
	Run: func(cmd *cobra.Command, args []string) {
		ifaceName, _ := cmd.Flags().GetString("iface")
		dnsTimeoutSeconds, _ := cmd.Flags().GetInt("dns-timeout")
		broadSearch, _ := cmd.Flags().GetBool("broad-search")
		
		var iface *net.Interface
		var err error
		if ifaceName != "" {
			if iface, err = net.InterfaceByName(ifaceName); err != nil {
				exit("unable to find interface %q: %v", ifaceName, err)
			}
		} else {
			// If no interface was specified, try to auto-detect the best interface
			if iface, err = detectBestInterface(); err != nil {
				// If auto-detection fails, continue without interface (original behavior)
				iface = nil
			}
		}
		
		if broadSearch {
			// Use hybrid approach: mDNS + port scanning
			foundDevices := performBroadSearch(iface, dnsTimeoutSeconds)
			if len(foundDevices) == 0 {
				outputError("no cast devices found on network")
			} else {
				for i, device := range foundDevices {
					outputInfo("%d) device=%q device_name=%q address=\"%s:%d\" uuid=%q", 
						i+1, device.Device, device.DeviceName, device.AddrV4, device.Port, device.UUID)
				}
			}
		} else {
			// Use original mDNS-only approach
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(dnsTimeoutSeconds))
			defer cancel()
			castEntryChan, err := castdns.DiscoverCastDNSEntries(ctx, iface)
			if err != nil {
				exit("unable to discover chromecast devices: %v", err)
			}
			i := 1
			for d := range castEntryChan {
				outputInfo("%d) device=%q device_name=%q address=\"%s:%d\" uuid=%q", i, d.Device, d.DeviceName, d.AddrV4, d.Port, d.UUID)
				i++
			}
			if i == 1 {
				outputError("no cast devices found on network")
			}
		}
	},
}

// CastDevice represents a discovered Chromecast device
type CastDevice struct {
	Device     string
	DeviceName string
	AddrV4     string
	Port       int
	UUID       string
}

// performBroadSearch does a comprehensive search using both mDNS and port scanning
func performBroadSearch(iface *net.Interface, dnsTimeoutSeconds int) []CastDevice {
	var allDevices []CastDevice
	deviceMap := make(map[string]CastDevice) // Use UUID as key to deduplicate
	
	// First, try mDNS discovery
	outputInfo("Performing mDNS discovery...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(dnsTimeoutSeconds*3)) // Use 3x timeout for broad search
	castEntryChan, err := castdns.DiscoverCastDNSEntries(ctx, iface)
	if err == nil {
		for d := range castEntryChan {
			device := CastDevice{
				Device:     d.Device,
				DeviceName: d.DeviceName,
				AddrV4:     d.AddrV4.String(),
				Port:       d.Port,
				UUID:       d.UUID,
			}
			if device.UUID != "" {
				deviceMap[device.UUID] = device
			} else {
				// If no UUID, use address:port as key
				key := fmt.Sprintf("%s:%d", device.AddrV4, device.Port)
				deviceMap[key] = device
			}
		}
	}
	cancel()
	
	outputInfo("Found %d devices via mDNS, performing port scan to find additional devices...", len(deviceMap))
	
	// Then, do a targeted port scan on the local subnet
	if localSubnet, err := detectLocalSubnet(""); err == nil {
		ipRange, err := ipaddr.NewIPAddressString(localSubnet).ToSequentialRange()
		if err == nil {
			// Use a smaller set of ports for ls to keep it reasonably fast
			ports := []int{8009, 8008, 8443, 32236} // Common ports + known group port
			
			var wg sync.WaitGroup
			ipCh := make(chan *ipaddr.IPAddress, 100)
			
			// Send IPs to scan
			go func() {
				it := ipRange.Iterator()
				for it.HasNext() {
					ip := it.Next()
					ipCh <- ip
				}
				close(ipCh)
			}()
			
			// Scan IPs in parallel
			for i := 0; i < 20; i++ { // Use fewer goroutines than scan command
				wg.Add(1)
				go func() {
					defer wg.Done()
					dialer := &net.Dialer{
						Timeout: 300 * time.Millisecond,
					}
					for ip := range ipCh {
						for _, port := range ports {
							conn, err := dialer.Dial("tcp", fmt.Sprintf("%v:%d", ip, port))
							if err != nil {
								continue
							}
							conn.Close()
							
							// Try to get device info
							if info, err := application.GetInfo(ip.String()); err == nil {
								device := CastDevice{
									Device:     "Unknown Device",
									DeviceName: info.Name,
									AddrV4:     ip.String(),
									Port:       port,
									UUID:       "", // Port scan doesn't give us UUID
								}
								
								// Use address:port as key since we don't have UUID from port scan
								key := fmt.Sprintf("%s:%d", device.AddrV4, device.Port)
								
								// Only add if we haven't seen this device yet
								if _, exists := deviceMap[key]; !exists {
									// Also check if we have this device by name on a different port
									found := false
									for _, existing := range deviceMap {
										if existing.DeviceName == device.DeviceName && existing.AddrV4 == device.AddrV4 {
											found = true
											break
										}
									}
									if !found {
										deviceMap[key] = device
									}
								}
							}
						}
					}
				}()
			}
			wg.Wait()
		}
	}
	
	// Convert map to slice and sort
	for _, device := range deviceMap {
		allDevices = append(allDevices, device)
	}
	
	// Sort by device name for consistent output
	sort.Slice(allDevices, func(i, j int) bool {
		return allDevices[i].DeviceName < allDevices[j].DeviceName
	})
	
	return allDevices
}

func init() {
	lsCmd.Flags().Bool("broad-search", false, "perform comprehensive search using both mDNS and port scanning")
	rootCmd.AddCommand(lsCmd)
}

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
		var (
			cidrAddr, _  = cmd.Flags().GetString("cidr")
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

func init() {
	scanCmd.Flags().String("cidr", "192.168.50.0/24", "cidr expression of subnet to scan")
	scanCmd.Flags().Int("port", 8009, "port to scan for")
	rootCmd.AddCommand(scanCmd)
}

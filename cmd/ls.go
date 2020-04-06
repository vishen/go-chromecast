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
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	castdns "github.com/vishen/go-chromecast/dns"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		ifaceName, _ := cmd.Flags().GetString("iface")
		dnsTimeoutSeconds, _ := cmd.Flags().GetInt("dns-timeout")
		var iface *net.Interface
		var err error
		if ifaceName != "" {
			if iface, err = net.InterfaceByName(ifaceName); err != nil {
				log.Fatalf("unable to find interface %q: %v", ifaceName, err)
			}
		}
		dnsEntries := castdns.FindCastDNSEntries(iface, dnsTimeoutSeconds)
		fmt.Printf("Found %d cast devices\n", len(dnsEntries))
		for i, d := range dnsEntries {
			fmt.Printf("%d) device=%q device_name=%q address=\"%s:%d\" status=%q uuid=%q\n", i+1, d.Device, d.DeviceName, d.AddrV4, d.Port, d.Status, d.UUID)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)
}

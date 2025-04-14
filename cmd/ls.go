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
	"net"
	"time"

	"github.com/spf13/cobra"
	castdns "github.com/vishen/go-chromecast/dns"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List devices",
	Run: func(cmd *cobra.Command, args []string) {
		ifaceName, _ := cmd.Flags().GetString("iface")
		dnsTimeoutSeconds, _ := cmd.Flags().GetInt("dns-timeout")
		Ls(ifaceName, dnsTimeoutSeconds)
	},
}

// Ls exports the ls command
func Ls(ifaceName string, dnsTimeoutSeconds int) {
	var (
		iface *net.Interface
		err   error
	)
	if ifaceName != "" {
		if iface, err = net.InterfaceByName(ifaceName); err != nil {
			exit("unable to find interface %q: %v", ifaceName, err)
		}
	}
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

func init() {
	rootCmd.AddCommand(lsCmd)
}

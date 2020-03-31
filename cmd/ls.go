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
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/discovery"
	"github.com/vishen/go-chromecast/discovery/zeroconf"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		devices, err := listDevices(3 * time.Second)
		if err != nil {
			return err
		}
		fmt.Printf("Found %d cast devices\n", len(devices))
		for i, d := range devices {
			fmt.Printf("%d) device=%q device_name=%q address=\"%s\" status=%q uuid=%q\n", i+1, d.Type(), d.Name(), d.Addr(), d.Status(), d.ID())
		}
		return nil
	},
}

func listDevices(scanDuration time.Duration) ([]*discovery.Device, error) {
	discover := discovery.Service{
		Scanner: zeroconf.Scanner{Logger: log.New()},
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, scanDuration)
	defer cancel()
	return discover.Sorted(ctx)
}

func init() {
	rootCmd.AddCommand(lsCmd)
}

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
	"time"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/log"
)

var (
	Version = ""
	Commit  = ""
	Date    = ""
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go-chromecast",
	Short: "CLI for interacting with the Google Chromecast",
	Long: `Control your Google Chromecast or Google Home Mini from the
command line.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		printVersion, _ := cmd.Flags().GetBool("version")
		if printVersion {
			if len(Version) > 0 && Version[0] != 'v' && Version != "dev" {
				Version = "v" + Version
			}
			log.Printf("go-chromecast %s (%s) %s", Version, Commit, Date)
			return nil
		}
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version, commit, date string) int {
	Version = version
	Commit = commit
	if date != "" {
		Date = date
	} else {
		Date = time.Now().UTC().Format(time.RFC3339)
	}
	if err := rootCmd.Execute(); err != nil {
		return 1
	}
	return 0
}

func init() {
	rootCmd.PersistentFlags().Bool("version", false, "display command version")
	// TODO: clean up shortened "v" for debug and move to verbose, and ensure
	// verbose is used appropriately as debug.
	rootCmd.PersistentFlags().BoolP("debug", "v", false, "debug logging")
	rootCmd.PersistentFlags().Bool("verbose", false, "verbose logging")
	rootCmd.PersistentFlags().Bool("disable-cache", false, "disable the cache")
	rootCmd.PersistentFlags().Bool("with-ui", false, "run with a UI")
	rootCmd.PersistentFlags().StringP("device", "d", "", "chromecast device, ie: 'Chromecast' or 'Google Home Mini'")
	rootCmd.PersistentFlags().StringP("device-name", "n", "", "chromecast device name")
	rootCmd.PersistentFlags().StringP("uuid", "u", "", "chromecast device uuid")
	rootCmd.PersistentFlags().StringP("addr", "a", "", "Address of the chromecast device")
	rootCmd.PersistentFlags().StringP("port", "p", "8009", "Port of the chromecast device if 'addr' is specified")
	rootCmd.PersistentFlags().StringP("iface", "i", "", "Network interface to use when looking for a local address to use for the http server or for use with multicast dns discovery")
	rootCmd.PersistentFlags().IntP("server-port", "s", 0, "Listening port for the http server")
	rootCmd.PersistentFlags().Int("dns-timeout", 3, "Multicast DNS timeout in seconds when searching for chromecast DNS entries")
	rootCmd.PersistentFlags().Bool("first", false, "Use first cast device found")
}

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
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go-chromecast",
	Short: "CLI for interacting with the Google Chromecast",
	Long: `Control your Google Chromecast or Google Home Mini from the
command line.
`}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("debug", false, "debug logging")
	rootCmd.PersistentFlags().Bool("disable-cache", false, "disable the cache")
	rootCmd.PersistentFlags().Bool("with-ui", false, "run with a UI")
	rootCmd.PersistentFlags().StringP("device", "d", "", "chromecast device, ie: 'Chromecast' or 'Google Home Mini'")
	rootCmd.PersistentFlags().StringP("device-name", "n", "", "chromecast device name")
	rootCmd.PersistentFlags().StringP("uuid", "u", "", "chromecast device uuid")
}

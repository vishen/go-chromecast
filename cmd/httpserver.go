// Copyright © 2020 Jonathan Pentecost <pentecostjonathan@gmail.com>
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
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/http"
)

// httpserverCmd represents the httpserver command
var httpserverCmd = &cobra.Command{
	Use:   "httpserver",
	Short: "Start the HTTP server",
	Long: `Start the HTTP server which provides an HTTP
api to control chromecast devices on a network.`,
	Run: func(cmd *cobra.Command, args []string) {

		addr, _ := cmd.Flags().GetString("http-addr")
		port, _ := cmd.Flags().GetString("http-port")

		// TODO: Should only need verbose, but debug has stupidly hijacked
		// the -v flag...
		verbose, _ := cmd.Flags().GetBool("verbose")
		debug, _ := cmd.Flags().GetBool("debug")

		if err := http.NewHandler(verbose || debug).Serve(addr + ":" + port); err != nil {
			exit("unable to run http server: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(httpserverCmd)
	httpserverCmd.Flags().String("http-port", "8011", "port for the http server to listen on")
	httpserverCmd.Flags().String("http-addr", "0.0.0.0", "addr for the http server to listen on")
}

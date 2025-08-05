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
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/application"
	"github.com/vishen/go-chromecast/ui"
)

// uiCmd represents the ui command (runs a UI):
var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Run the UI",
	Run: func(cmd *cobra.Command, args []string) {
		broadSearch, _ := cmd.Flags().GetBool("broad-search")
		
		var app application.App
		var err error
		
		if broadSearch {
			// Use broad search for device discovery
			app, err = castApplicationWithBroadSearch(cmd, args)
		} else {
			// Use standard device discovery
			app, err = castApplication(cmd, args)
		}
		
		if err != nil {
			exit("unable to get cast application: %v", err)
			return
		}

		ccui, err := ui.NewUserInterface(app)
		if err != nil {
			exit("unable to prepare a new user-interface: %v", err)
		}

		if err := ccui.Run(); err != nil {
			exit("unable to start the user-interface: %v", err)
		}
	},
}

func init() {
	uiCmd.Flags().Bool("broad-search", false, "perform comprehensive search using both mDNS and port scanning")
	rootCmd.AddCommand(uiCmd)
}

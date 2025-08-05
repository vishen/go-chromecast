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
)

// togglepauseCmd represents the togglepause command
var togglepauseCmd = &cobra.Command{
	Use:     "togglepause",
	Aliases: []string{"tpause", "playpause"},
	Short:   "Toggle paused/unpaused state. Aliases: tpause, playpause",
	Run: func(cmd *cobra.Command, args []string) {
		app, err := castApplication(cmd, args)
		if err != nil {
			exit("unable to get cast application: %v", err)
		}
		if err := app.TogglePause(); err != nil {
			exit("unable to (un)pause cast application: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(togglepauseCmd)
	togglepauseCmd.Flags().BoolP("broad-search", "b", false, "Search for devices using comprehensive network scanning (slower but finds more devices)")
}

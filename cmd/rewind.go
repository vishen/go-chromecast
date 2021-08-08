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
	"strconv"

	"github.com/spf13/cobra"
)

// rewindCmd represents the rewind command
var rewindCmd = &cobra.Command{
	Use:   "rewind <delta_in_seconds>",
	Short: "Rewind by seconds the currently playing media",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			exit("one argument required\n")
		}
		value, err := strconv.Atoi(args[0])
		if err != nil {
			exit("unable to parse %q to an integer\n", args[0])
		}
		app, err := castApplication(cmd, args)
		if err != nil {
			exit("unable to get cast application: %v\n", err)
		}
		if err := app.Seek(-value); err != nil {
			exit("unable to rewind current media: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(rewindCmd)
}

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
	"github.com/vishen/go-chromecast/log"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop casting",
	Run: func(cmd *cobra.Command, args []string) {
		app, err := castApplication(cmd, args)
		if err != nil {
			exit("unable to get cast application: %v\n", err)
		}
		if err := app.Stop(); err != nil {
			exit("unable to stop casting: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

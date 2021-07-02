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
	"github.com/vishen/go-chromecast/log"
)

// muteCmd represents the mute command
var muteCmd = &cobra.Command{
	Use:   "mute",
	Short: "Mute the chromecast",
	Run: func(cmd *cobra.Command, args []string) {
		app, err := castApplication(cmd, args)
		if err != nil {
			log.WithError(err).Error("unable to get cast application")
			return
		}
		if err := app.SetMuted(true); err != nil {
			log.WithError(err).Error("unable to mute cast application")
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(muteCmd)
}

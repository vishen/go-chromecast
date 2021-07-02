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
	"github.com/vishen/go-chromecast/log"
	"github.com/vishen/go-chromecast/ui"

	"github.com/spf13/cobra"
)

// uiCmd represents the ui command (runs a UI):
var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Run the UI",
	Run: func(cmd *cobra.Command, args []string) {
		app, err := castApplication(cmd, args)
		if err != nil {
			log.WithError(err).Error("unable to get cast application")
			return
		}

		ccui, err := ui.NewUserInterface(app)
		if err != nil {
			log.WithError(err).Error("unable to prepare a new user-interface")
		}

		if err := ccui.Run(); err != nil {
			log.WithError(err).Error("unable to start the user-interface")
		}
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}

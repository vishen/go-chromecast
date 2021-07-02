// Copyright © 2021 Jonathan Pentecost <pentecostjonathan@gmail.com>
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

	"github.com/vishen/go-chromecast/ui"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/log"
)

// loadAppCmd represents the load command
var loadAppCmd = &cobra.Command{
	Use:   "load-app <app-id> <content-id>",
	Short: "Load and play content on a chromecast app",
	Long: `Load and play content on a chromecast app. This requires
the chromecast receiver app to be specified. An older list can be found 
here https://gist.github.com/jloutsenhizer/8855258.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("requires exactly two arguments")
		}
		app, err := castApplication(cmd, args)
		if err != nil {
			log.WithError(err).Info("unable to get cast application")
			return nil
		}

		// Optionally run a UI when playing this media:
		runWithUI, _ := cmd.Flags().GetBool("with-ui")
		if runWithUI {
			go func() {
				if err := app.LoadApp(args[0], args[1]); err != nil {
					log.WithError(err).Fatal("unable to load media")
				}
			}()

			ccui, err := ui.NewUserInterface(app)
			if err != nil {
				log.WithError(err).Fatal("unable to prepare a new user-interface")
			}
			return ccui.Run()
		}

		// Otherwise just run in CLI mode:
		if err := app.LoadApp(args[0], args[1]); err != nil {
			log.WithError(err).Info("unable to load media")
			return nil
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loadAppCmd)
}

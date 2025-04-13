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
	"strings"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/ui"
)

// transcodeCmd represents the transcode command
var transcodeCmd = &cobra.Command{
	Use:   "transcode",
	Short: "Transcode and play media on the chromecast",
	Long: `Transcode and play media on the chromecast. This will start a streaming server
locally and serve the output of the transcoding operation to the chromecast. 
This command requires the program or script to write the media content to stdout.
The transcoded media content-type is required as well`,
	Run: func(cmd *cobra.Command, args []string) {
		contentType, _ := cmd.Flags().GetString("content-type")
		command, _ := cmd.Flags().GetString("command")
		runWithUI, _ := cmd.Flags().GetBool("with-ui")

		app := NewCast(cmd)
		app.Transcode(contentType, command, runWithUI, args)
	},
}

// Transcode exports the transcode command
func (a *App) Transcode(contentType, command string, runWithUI bool, args []string) {
	app, err := a.castApplication()
	if err != nil {
		exit("unable to get cast application: %v", err)
	}

	var commandArgs []string
	if command == "" {
		command = args[0]
		commandArgs = args[1:]
	} else {
		s := strings.Split(command, " ")
		command = s[0]
		commandArgs = s[1:]
	}

	if runWithUI {
		go func() {
			if err := app.Transcode(contentType, command, commandArgs...); err != nil {
				exit("unable to load media: %v", err)
			}
		}()

		ccui, err := ui.NewUserInterface(app)
		if err != nil {
			exit("unable to prepare a new user-interface: %v", err)
		}
		if err := ccui.Run(); err != nil {
			exit("unable to run ui: %v", err)
		}
	}

	if err := app.Transcode(contentType, command, commandArgs...); err != nil {
		exit("unable to transcode media: %v", err)
	}
}

func init() {
	rootCmd.AddCommand(transcodeCmd)
	transcodeCmd.Flags().String("command", "", "command to use when transcoding")
	transcodeCmd.Flags().StringP("content-type", "c", "", "content-type to serve the media file as")
}

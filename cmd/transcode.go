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
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := castApplication(cmd, args)
		if err != nil {
			log.WithError(err).Errorf("unable to get cast application")
			return nil
		}

		contentType, _ := cmd.Flags().GetString("content-type")
		command, _ := cmd.Flags().GetString("command")

		runWithUI, _ := cmd.Flags().GetBool("with-ui")
		if runWithUI {
			go func() {
				if err := app.Transcode(command, contentType); err != nil {
					log.WithError(err).Fatal("unable to load media")
				}
			}()

			ccui, err := ui.NewUserInterface(app)
			if err != nil {
				log.WithError(err).Fatal("unable to prepare a new user-interface")
			}
			return ccui.Run()
		}

		if err := app.Transcode(command, contentType); err != nil {
			log.WithError(err).Errorf("unable to transcode media")
			return nil
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(transcodeCmd)
	transcodeCmd.Flags().String("command", "", "command to use when transcoding")
	transcodeCmd.Flags().StringP("content-type", "c", "", "content-type to serve the media file as")
}

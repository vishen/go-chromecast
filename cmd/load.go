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
	"fmt"

	"github.com/vishen/go-chromecast/ui"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/log"
)

// loadCmd represents the load command
var loadCmd = &cobra.Command{
	Use:   "load <filename_or_url>",
	Short: "Load and play media on the chromecast",
	Long: `Load and play media files on the chromecast, this will
start a HTTP server locally and will stream the media file to the
chromecast if it is a local file, otherwise it will load the url.

If the media file is an unplayable media type by the chromecast, this
will attempt to transcode the media file to mp4 using ffmpeg. This requires
that ffmpeg is installed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly one argument, should be the media file to load")
		}
		app, err := castApplication(cmd, args)
		if err != nil {
			log.WithError(err).Error("unable to get cast application")
			return nil
		}

		contentType, _ := cmd.Flags().GetString("content-type")
		transcode, _ := cmd.Flags().GetBool("transcode")
		detach, _ := cmd.Flags().GetBool("detach")

		// Optionally run a UI when playing this media:
		runWithUI, _ := cmd.Flags().GetBool("with-ui")
		if runWithUI {
			go func() {
				if err := app.Load(args[0], contentType, transcode, detach, false); err != nil {
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
		if err := app.Load(args[0], contentType, transcode, detach, false); err != nil {
			log.WithError(err).Error("unable to load media")
			return nil
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)
	loadCmd.Flags().Bool("transcode", true, "transcode the media to mp4 if media type is unrecognised")
	loadCmd.Flags().Bool("detach", false, "detach from waiting until media finished. Only works with url loaded external media")
	loadCmd.Flags().StringP("content-type", "c", "", "content-type to serve the media file as")
}

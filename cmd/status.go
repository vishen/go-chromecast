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

	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Current chromecast status",
	Run: func(cmd *cobra.Command, args []string) {
		app, err := castApplication(cmd, args)
		if err != nil {
			exit("unable to get cast application: %v", err)
		}
		castApplication, castMedia, castVolume := app.Status()
		volumeLevel := castVolume.Level
		volumeMuted := castVolume.Muted

		contentId, _ := cmd.Flags().GetBool("content-id")

		scriptMode := contentId

		if scriptMode {
			if contentId {
				if castMedia != nil {
					outputInfo(castMedia.Media.ContentId)
				} else {
					outputInfo("not available")
				}
			}
		} else if castApplication == nil {
			outputInfo("Idle, volume=%0.2f muted=%t", volumeLevel, volumeMuted)
		} else {
			displayName := castApplication.DisplayName
			if castApplication.IsIdleScreen {
				outputInfo("Idle (%s), volume=%0.2f muted=%t", displayName, volumeLevel, volumeMuted)
			} else if castMedia == nil {
				outputInfo("Idle (%s), volume=%0.2f muted=%t", displayName, volumeLevel, volumeMuted)
			} else {
				var metadata string
				var usefulID string
				switch castMedia.Media.ContentType {
				case "x-youtube/video":
					usefulID = fmt.Sprintf("[%s] ", castMedia.Media.ContentId)
				}
				if castMedia.Media.Metadata.Title != "" {
					md := castMedia.Media.Metadata
					metadata = fmt.Sprintf("title=%q, artist=%q", md.Title, md.Artist)
				}
				if castMedia.Media.ContentId != "" {
					if metadata != "" {
						metadata += ", "
					}
					metadata += fmt.Sprintf("[%s]", castMedia.Media.ContentId)
				}
				if metadata == "" {
					metadata = "unknown"

				}
				outputInfo("%s%s (%s), %s, time remaining=%.0fs/%.0fs, volume=%0.2f, muted=%t", usefulID, displayName, castMedia.PlayerState, metadata, castMedia.CurrentTime, castMedia.Media.Duration, volumeLevel, volumeMuted)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("content-id", false, "print the content id if available")
}

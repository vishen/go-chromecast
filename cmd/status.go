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
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := castApplication(cmd, args)
		if err != nil {
			return err
		}
		if err := app.Update(); err != nil {
			return err
		}
		castApplication, castMedia, castVolume := app.Status()
		if castApplication == nil {
			fmt.Printf("Idle, volume=%0.2f muted=%t\n", castVolume.Level, castVolume.Muted)
		} else if castApplication.IsIdleScreen {
			fmt.Printf("Idle (%s), volume=%0.2f muted=%t\n", castApplication.DisplayName, castVolume.Level, castVolume.Muted)
		} else {
			metadata := "unknown"
			if castMedia.Media.Metadata.Title != "" {
				md := castMedia.Media.Metadata
				metadata = fmt.Sprintf("title=%q, artist=%q", md.Title, md.Artist)
			}
			fmt.Printf("%s (%s), %s, time remaining=%.0fs/%.0fs, volume=%0.2f, muted=%t\n", castApplication.DisplayName, castMedia.PlayerState, metadata, castMedia.CurrentTime, castMedia.Media.Duration, castVolume.Level, castVolume.Muted)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

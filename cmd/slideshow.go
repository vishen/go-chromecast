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
	"os"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/log"
)

// slideshowCmd represents the slideshow command
var slideshowCmd = &cobra.Command{
	Use:   "slideshow file1 file2 ...",
	Short: "Play a slideshow of photos",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("requires files to play in slideshow")
		}
		for _, arg := range args {
			if fileInfo, err := os.Stat(arg); err != nil {
				log.WithError(err).Errorf("unable to find %q", arg)
				return nil
			} else if fileInfo.Mode().IsDir() {
				log.Printf("%q is a directory", arg)
				return nil
			}
		}
		app, err := castApplication(cmd, args)
		if err != nil {
			log.WithError(err).Errorf("unable to get cast application")
			return nil
		}

		duration, _ := cmd.Flags().GetInt("duration")
		repeat, _ := cmd.Flags().GetBool("repeat")
		if err := app.Slideshow(args, duration, repeat); err != nil {
			log.WithError(err).Errorf("unable to play slideshow on cast application")
			return nil
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(slideshowCmd)
	slideshowCmd.Flags().Int("duration", 10, "duration of each image on screen")
	slideshowCmd.Flags().Bool("repeat", true, "should the slideshow repeat")
}

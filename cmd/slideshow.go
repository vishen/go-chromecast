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
	"os"

	"github.com/spf13/cobra"
)

// slideshowCmd represents the slideshow command
var slideshowCmd = &cobra.Command{
	Use:   "slideshow file1 file2 ...",
	Short: "Play a slideshow of photos",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			exit("requires files to play in slideshow")
		}
		for _, arg := range args {
			if fileInfo, err := os.Stat(arg); err != nil {
				exit("unable to find %q: %v", arg, err)
			} else if fileInfo.Mode().IsDir() {
				exit("%q is a directory", arg)
			}
		}
		app, err := castApplication(cmd, args)
		if err != nil {
			exit("unable to get cast application: %v", err)
		}

		duration, _ := cmd.Flags().GetInt("duration")
		repeat, _ := cmd.Flags().GetBool("repeat")
		if err := app.Slideshow(args, duration, repeat); err != nil {
			exit("unable to play slideshow on cast application: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(slideshowCmd)
	slideshowCmd.Flags().Int("duration", 10, "duration of each image on screen")
	slideshowCmd.Flags().Bool("repeat", true, "should the slideshow repeat")
}

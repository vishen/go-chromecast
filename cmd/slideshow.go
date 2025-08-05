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

	"io/fs"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"path/filepath"
	"strings"
)

type share struct {
	files []string
}

// slideshowCmd represents the slideshow command
var slideshowCmd = &cobra.Command{
	Use:   "slideshow file1 file2 ...",
	Short: "Play a slideshow of photos",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			exit("requires files (or directories) to play in slideshow")
		}

		s := &share{}

		for _, arg := range args {
			if fileInfo, err := os.Stat(arg); err != nil {
				log.Warn().Msgf("unable to find %q: %v", arg, err)
			} else if fileInfo.Mode().IsDir() {
				log.Debug().Msgf("%q is a directory", arg)

				// recursively find files in directory
				// TODO: this will consume large amounts of memory as it will hold references to each file (media item) to be served
				filepath.WalkDir(arg, s.getFilesRecursively)
			} else {
				s.files = append(s.files, arg)
			}
		}

		app, err := castApplication(cmd, s.files)
		if err != nil {
			exit("unable to get cast application: %v", err)
		}

		duration, _ := cmd.Flags().GetInt("duration")
		repeat, _ := cmd.Flags().GetBool("repeat")
		if err := app.Slideshow(s.files, duration, repeat); err != nil {
			exit("unable to play slideshow on cast application: %v", err)
		}
	},
}

func (s *share) getFilesRecursively(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		log.Warn().Msgf("Error checking file: %v | %v", path, err)
		return nil
	}

	if !fileInfo.Mode().IsDir() {
		if isSupportedImageType(path) {
			s.files = append(s.files, path)
		} else {
			log.Warn().Msgf("excluding %s as it is not a supported image type", path)
		}
	}

	return nil
}

func isSupportedImageType(path string) bool {
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".jpg", ".jpeg", ".gif", ".bmp", ".png", ".webp":
		return true
	default:
		return false
	}
}

func init() {
	rootCmd.AddCommand(slideshowCmd)
	slideshowCmd.Flags().Int("duration", 10, "duration of each image on screen")
	slideshowCmd.Flags().Bool("repeat", true, "should the slideshow repeat")
	slideshowCmd.Flags().BoolP("broad-search", "b", false, "Search for devices using comprehensive network scanning (slower but finds more devices)")
}

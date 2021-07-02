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
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/log"
	"github.com/vishen/go-chromecast/ui"
)

type mediaFile struct {
	filename        string
	possibleNumbers []int
}

// playlistCmd represents the playlist command
var playlistCmd = &cobra.Command{
	Use:   "playlist <directory>",
	Short: "Load and play media on the chromecast",
	Long: `Load and play media files on the chromecast, this will
start a streaming server locally and serve the media file to the
chromecast.

If the media file is an unplayable media type by the chromecast, this
will attempt to transcode the media file to mp4 using ffmpeg. This requires
that ffmpeg is installed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly one argument, should be the folder to play media from")
		}
		if fileInfo, err := os.Stat(args[0]); err != nil {
			log.WithError(err).Errorf("unable to find %q", args[0])
			return nil
		} else if !fileInfo.Mode().IsDir() {
			log.Printf("%q is not a directory", args[0])
			return nil
		}
		app, err := castApplication(cmd, args)
		if err != nil {
			log.WithError(err).Error("unable to get cast application")
			return nil
		}

		contentType, _ := cmd.Flags().GetString("content-type")
		transcode, _ := cmd.Flags().GetBool("transcode")
		forcePlay, _ := cmd.Flags().GetBool("force-play")
		continuePlaying, _ := cmd.Flags().GetBool("continue")
		selection, _ := cmd.Flags().GetBool("select")
		files, err := ioutil.ReadDir(args[0])
		if err != nil {
			log.WithError(err).Errorf("unable to list files from %q", args[0])
			return nil
		}
		filesToPlay := make([]mediaFile, 0, len(files))
		for _, f := range files {
			if !forcePlay && !app.PlayableMediaType(f.Name()) {
				continue
			}

			foundNum := false
			numPos := 0
			foundNumbers := []int{}
			for i, c := range f.Name() {
				if c < '0' || c > '9' {
					if foundNum {
						val, _ := strconv.Atoi(f.Name()[numPos:i])
						foundNumbers = append(foundNumbers, val)
					}
					foundNum = false
					continue
				}

				if !foundNum {
					numPos = i
					foundNum = true
				}
			}

			filesToPlay = append(filesToPlay, mediaFile{
				filename:        f.Name(),
				possibleNumbers: foundNumbers,
			})

		}

		sort.Slice(filesToPlay, func(i, j int) bool {
			iNum := filesToPlay[i].possibleNumbers
			jNum := filesToPlay[j].possibleNumbers
			if len(iNum) == 0 {
				return false
			}
			if len(jNum) == 0 {
				return true
			}
			max := len(iNum)
			if len(iNum) < len(jNum) {
				max = len(jNum)
			}
			for vi := 0; vi < max; vi++ {
				if len(iNum) < vi {
					return false
				}
				if len(jNum) < vi {
					return true
				}
				if iNum[vi] == jNum[vi] {
					continue
				}
				if iNum[vi] > jNum[vi] {
					return false
				}
				return true
			}
			return true
		})

		filenames := make([]string, len(filesToPlay))
		for i, f := range filesToPlay {
			filename := filepath.Join(args[0], f.filename)
			filenames[i] = filename
		}

		indexToPlayFrom := 0
		if selection {
			log.Print("Will play the following items, select where to start from:")
			for i, f := range filenames {
				lastPlayed := "never"
				if lp, ok := app.PlayedItems()[f]; ok {
					t := time.Unix(lp.Started, 0)
					lastPlayed = t.String()
				}
				log.Printf("%d) %s: last played %q", i+1, f, lastPlayed)
			}
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Printf("Enter selection: ")
				text, err := reader.ReadString('\n')
				if err != nil {
					fmt.Printf("error reading console: %v", err)
					continue
				}
				i, err := strconv.Atoi(strings.TrimSpace(text))
				if err != nil {
					continue
				} else if i < 1 || i > len(filenames) {
					continue
				}
				indexToPlayFrom = i - 1
				break
			}
		} else if continuePlaying {
			var lastPlayedStartUnix int64 = 0
			var lastPlayedEndUnix int64 = 0
			lastPlayedIndex := 0
			for i, f := range filenames {
				p, ok := app.PlayedItems()[f]
				if ok && p.Started > lastPlayedStartUnix {
					lastPlayedStartUnix = p.Started
					lastPlayedEndUnix = p.Finished
					lastPlayedIndex = i
				}
			}

			if lastPlayedIndex > 0 {
				if lastPlayedStartUnix < lastPlayedEndUnix {
					if len(filenames) > lastPlayedIndex {
						// lastPlayedIndex += 1
					} else {
						lastPlayedIndex = 0
					}
				}
			}
			indexToPlayFrom = lastPlayedIndex
		}

		s := "Attemping to play the following media:"
		for _, f := range filenames[indexToPlayFrom:] {
			s += "- " + f + "\n"
		}
		log.Print(s)

		// Optionally run a UI when playing this media:
		runWithUI, _ := cmd.Flags().GetBool("with-ui")
		if runWithUI {
			go func() {
				if err := app.QueueLoad(filenames[indexToPlayFrom:], contentType, transcode); err != nil {
					log.WithError(err).Fatal("unable to play playlist on cast application")
				}
			}()

			ccui, err := ui.NewUserInterface(app)
			if err != nil {
				log.WithError(err).Fatal("unable to prepare a new user-interface")
			}
			return ccui.Run()
		}

		if err := app.QueueLoad(filenames[indexToPlayFrom:], contentType, transcode); err != nil {
			log.WithError(err).Error("unable to play playlist on cast application")
			return nil
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(playlistCmd)
	playlistCmd.Flags().Bool("continue", true, "continue playing from the last known media")
	playlistCmd.Flags().Bool("select", false, "choose which media to start the playlist from")
	playlistCmd.Flags().Bool("transcode", true, "transcode the media to mp4 if media type is unrecognised")
	playlistCmd.Flags().Bool("force-play", false, "attempt to play a media type even if it is unrecognised")
	playlistCmd.Flags().StringP("content-type", "c", "", "content-type to serve the media file as")
}

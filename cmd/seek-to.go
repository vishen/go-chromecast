// Copyright © 2020 Jonathan Pentecost <pentecostjonathan@gmail.com>
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
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/log"
)

// seekToCmd represents the seekTo command
var seekToCmd = &cobra.Command{
	Use:   "seek-to <timestamp_in_seconds>",
	Short: "Seek to the <timestamp_in_seconds> in the currently playing media",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("one argument required")
		}
		value, err := strconv.ParseFloat(args[0], 32)
		if err != nil {
			log.WithError(err).Errorf("unable to parse %q to an integer", args[0])
			return nil
		}
		app, err := castApplication(cmd, args)
		if err != nil {
			log.WithError(err).Error("unable to get cast application")
			return nil
		}
		if err := app.SeekToTime(float32(value)); err != nil {
			log.WithError(err).Error("unable to seek to current media")
			return nil
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(seekToCmd)
}

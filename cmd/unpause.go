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
)

// unpauseCmd represents the unpause command
var unpauseCmd = &cobra.Command{
	Use:   "unpause",
	Short: "Unpause the currently playing media on the chromecast",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := castApplication(cmd, args)
		if err != nil {
			return err
		}
		if err := app.Unpause(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(unpauseCmd)
}

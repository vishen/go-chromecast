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
	"math"

	"github.com/spf13/cobra"
)

// nextCmd represents the next command
var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Play the next available media",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := castApplication(cmd, args)
		if err != nil {
			return err
		}
		// TODO(vishen): There must be a better way that this
		if err := app.Seek(math.MaxInt64); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(nextCmd)
}

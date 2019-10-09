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
	"strconv"

	"github.com/spf13/cobra"
)

// volumeCmd represents the volume command
var volumeCmd = &cobra.Command{
	Use:   "volume [<0.00 - 1.00>]",
	Short: "Get or set volume",
	Long:  "Get or set volume (float in range from 0 to 1)",
	Run: func(cmd *cobra.Command, args []string) {
		app, err := castApplication(cmd, args)
		if err != nil {
			fmt.Printf("unable to get cast application: %v\n", err)
			return
		}

		if len(args) == 1 && args[0] != "" {
			newVolume, err := strconv.ParseFloat(args[0], 32)
			if err != nil {
				fmt.Printf("invalid volume: %v\n", err)
				return
			}
			if err = app.SetVolume(float32(newVolume)); err != nil {
				fmt.Printf("failed to set volume: %v\n", err)
				return
			}
		}

		if err = app.Update(); err != nil {
			fmt.Printf("unable to update cast info: %v\n", err)
			return
		}
		_, _, castVolume := app.Status()

		fmt.Printf("%0.2f\n", castVolume.Level)

		return
	},
}

func init() {
	rootCmd.AddCommand(volumeCmd)
}

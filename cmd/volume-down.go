// Copyright © 2025 leak4mk0
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

// volumeDownCmd represents the volume-down command
var volumeDownCmd = &cobra.Command{
	Use:   "volume-down",
	Short: "Turn down volume",
	Run: func(cmd *cobra.Command, args []string) {
		app, err := castApplication(cmd, args)
		if err != nil {
			exit("unable to get cast application: %v", err)
		}

		volumeStep, _ := cmd.Flags().GetFloat32("step")

		if err = app.Update(); err != nil {
			exit("unable to update cast info: %v", err)
		}
		_, _, castVolume := app.Status()

		nextVolume := max(castVolume.Level-volumeStep, math.SmallestNonzeroFloat32)
		if err = app.SetVolume(float32(nextVolume)); err != nil {
			exit("failed to set volume: %v", err)
		}

		if err = app.Update(); err != nil {
			exit("unable to update cast info: %v", err)
		}
		_, _, turnedCastVolume := app.Status()

		outputInfo("%0.2f", turnedCastVolume.Level)
	},
}

func init() {
	rootCmd.AddCommand(volumeDownCmd)
	volumeDownCmd.Flags().Float32("step", 0.05, "step value for turning down volume")
	volumeDownCmd.Flags().BoolP("broad-search", "b", false, "Search for devices using comprehensive network scanning (slower but finds more devices)")
}

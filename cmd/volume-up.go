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
	"github.com/spf13/cobra"
)

// volumeUpCmd represents the volume-up command
var volumeUpCmd = &cobra.Command{
	Use:   "volume-up",
	Short: "Turn up volume",
	Run: func(cmd *cobra.Command, args []string) {
		volumeStep, _ := cmd.Flags().GetFloat32("step")

		app := NewCast(cmd)
		app.VolumeUp(volumeStep)
	},
}

// VolumeUp exports the volume-up command
func (a *App) VolumeUp(volumeStep float32) {
	app, err := a.castApplication()
	if err != nil {
		exit("unable to get cast application: %v", err)
	}

	if err = app.Update(); err != nil {
		exit("unable to update cast info: %v", err)
	}
	_, _, castVolume := app.Status()

	nextVolume := min(castVolume.Level+volumeStep, 1)
	if err = app.SetVolume(float32(nextVolume)); err != nil {
		exit("failed to set volume: %v", err)
	}

	if err = app.Update(); err != nil {
		exit("unable to update cast info: %v", err)
	}
	_, _, turnedCastVolume := app.Status()

	outputInfo("%0.2f", turnedCastVolume.Level)
}

func init() {
	rootCmd.AddCommand(volumeUpCmd)
	volumeUpCmd.Flags().Float32("step", 0.05, "step value for turning up volume")
}

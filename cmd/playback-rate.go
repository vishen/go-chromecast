package cmd

import (
	"strconv"

	"github.com/spf13/cobra"
)

// playbackRateCmd represents the playback-rate command
var playbackRateCmd = &cobra.Command{
	Use:   "playback-rate <rate>",
	Short: "Set the playback rate of the currently playing media",
	Long: `Set the playback rate (speed) of the currently playing media, where 1
is the normal speed. The rate must be between 0.5 and 2.

Not all media and cast applications support changing the playback rate.`,
	Example: `  go-chromecast playback-rate 1.5
  go-chromecast playback-rate 1`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			exit("requires exactly one argument, the playback rate")
		}
		rate, err := strconv.ParseFloat(args[0], 32)
		if err != nil {
			exit("invalid playback rate %q: %v", args[0], err)
		}
		app, err := castApplication(cmd, args)
		if err != nil {
			exit("unable to get cast application: %v", err)
		}
		if err := app.SetPlaybackRate(float32(rate)); err != nil {
			exit("unable to set the playback rate: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(playbackRateCmd)
}

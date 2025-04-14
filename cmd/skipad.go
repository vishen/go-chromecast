package cmd

import (
	"github.com/spf13/cobra"
)

// skipadCmd represents the skipad command
var skipadCmd = &cobra.Command{
	Use:   "skipad",
	Short: "Skip the currently playing ad on the chromecast",
	Run: func(cmd *cobra.Command, args []string) {
		app := NewCast(cmd)
		app.Skipad()
	},
}

// Skipad exports the skipad command
func (a *App) Skipad() {
	app, err := a.castApplication()
	if err != nil {
		exit("unable to get cast application: %v\n", err)
		return
	}
	if err := app.Skipad(); err != nil {
		exit("unable to skip current ad: %v\n", err)
	}
}

func init() {
	rootCmd.AddCommand(skipadCmd)
}

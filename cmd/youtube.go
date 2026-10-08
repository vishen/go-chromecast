package cmd

import (
	"slices"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/youtube"
)

// youtubeCmd represents the youtube command
var youtubeCmd = &cobra.Command{
	Use:   "youtube <url-or-id> [<url-or-id>...]",
	Short: "Play YouTube videos on the YouTube app",
	Long: `Play YouTube videos (or a playlist) on the YouTube app of the
chromecast. The videos can be YouTube urls or just the video id.

The first video replaces the current queue and starts playing, and any
other videos are added to the queue. Use --add or --next to add all of the
videos to the existing queue instead.

This uses the same unofficial API the YouTube mobile apps use to cast
to a device, so it needs access to youtube.com.`,
	Example: `  go-chromecast youtube https://www.youtube.com/watch?v=dQw4w9WgXcQ
  go-chromecast youtube dQw4w9WgXcQ
  go-chromecast youtube --add https://youtu.be/dQw4w9WgXcQ
  go-chromecast youtube "https://www.youtube.com/playlist?list=PL..."`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			exit("requires at least one youtube url or video id")
		}
		add, _ := cmd.Flags().GetBool("add")
		next, _ := cmd.Flags().GetBool("next")
		if add && next {
			exit("only one of --add and --next can be used")
		}
		queueOnly := add || next

		type video struct{ videoID, playlistID string }
		videos := make([]video, len(args))
		for i, arg := range args {
			videoID, playlistID, err := youtube.ParseURL(arg)
			if err != nil {
				exit("%v", err)
			}
			if videoID == "" && (queueOnly || i > 0) {
				exit("%q is a playlist, only videos can be added to the queue", arg)
			}
			videos[i] = video{videoID, playlistID}
		}

		app, err := castApplication(cmd, args)
		if err != nil {
			exit("unable to get cast application: %v", err)
		}

		if next {
			// Each video is inserted just after the current one, so
			// insert them in reverse to keep the order given.
			slices.Reverse(videos)
		}
		for i, v := range videos {
			if i == 0 && !queueOnly {
				if err := app.LoadYouTube(v.videoID, v.playlistID); err != nil {
					exit("unable to play youtube video: %v", err)
				}
				continue
			}
			if err := app.QueueYouTube(v.videoID, next); err != nil {
				exit("unable to add youtube video to the queue: %v", err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(youtubeCmd)
	youtubeCmd.Flags().Bool("add", false, "add the videos to the end of the queue instead of playing them now")
	youtubeCmd.Flags().Bool("next", false, "add the videos to the queue to play after the current video")
}

package application

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/vishen/go-chromecast/cast"
)

// subtitlesTrackID is the id of the text track with the subtitles. There is
// only ever one track, so any id will do.
const subtitlesTrackID = 1

// WithSubtitles sets the subtitles to show with the media that is loaded:
// either an url of a WebVTT file, or a local file. A local file that isn't
// WebVTT (ie: srt or ass, or even a video with embedded subtitles) is
// converted with ffmpeg.
func WithSubtitles(subtitles string) ApplicationOption {
	return func(a *Application) {
		a.subtitles = subtitles
	}
}

// subtitlesTracks returns the text track for the subtitles that were set, if
// any, making sure they are being served if they are a local file.
func (a *Application) subtitlesTracks() ([]cast.MediaTrack, error) {
	if a.subtitles == "" {
		return nil, nil
	}

	url := a.subtitles
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		if _, err := os.Stat(a.subtitles); err != nil {
			return nil, errors.Wrapf(err, "unable to find subtitles %q", a.subtitles)
		}
		localIP, err := a.getLocalIP()
		if err != nil {
			return nil, err
		}
		if err := a.startStreamingServer(); err != nil {
			return nil, errors.Wrap(err, "unable to start streaming server")
		}
		url = fmt.Sprintf("http://%s:%d%s", localIP, a.serverPort, subtitlesPath)
	}

	return []cast.MediaTrack{{
		TrackId:          subtitlesTrackID,
		Type:             "TEXT",
		Subtype:          "SUBTITLES",
		Name:             "Subtitles",
		TrackContentId:   url,
		TrackContentType: "text/vtt",
	}}, nil
}

// subtitlesTextTrackStyle is how the subtitles are shown: white text with a
// black outline and no background, which is readable on top of any video.
var subtitlesTextTrackStyle = cast.TextTrackStyle{
	BackgroundColor: "#00000000",
	ForegroundColor: "#FFFFFFFF",
	EdgeType:        "OUTLINE",
	EdgeColor:       "#000000FF",
}

// subtitlesPath is where the streaming server serves the subtitles.
const subtitlesPath = "/subtitles"

// serveSubtitles sends the subtitles as WebVTT, which is the only format
// that every cast device understands.
func (a *Application) serveSubtitles(w http.ResponseWriter, r *http.Request) {
	a.log("serving subtitles %q", a.subtitles)
	if a.subtitles == "" {
		http.NotFound(w, r)
		return
	}

	// The cast device won't load the subtitles without this.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")

	if strings.EqualFold(path.Ext(a.subtitles), ".vtt") {
		http.ServeFile(w, r, a.subtitles)
		return
	}

	webvtt, err := exec.Command(
		"ffmpeg",
		"-v", "error",
		"-i", a.subtitles,
		"-map", "0:s:0", // the first subtitles, in case there are more
		"-f", "webvtt",
		"pipe:1",
	).Output()
	if err != nil {
		a.log("error converting subtitles %q: %v", a.subtitles, err)
		w.Header().Del("Content-Type")
		http.Error(w, "unable to convert the subtitles to WebVTT", http.StatusInternalServerError)
		return
	}
	w.Write(webvtt)
}

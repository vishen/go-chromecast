package application

import (
	"bytes"
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

// WithSubtitlesTrack sets which subtitles to use when the file given to
// WithSubtitles has more than one, starting from 1. It defaults to the first.
func WithSubtitlesTrack(track int) ApplicationOption {
	return func(a *Application) {
		a.subtitlesTrack = track
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
		// Convert them now, and not when the chromecast asks for them, so
		// that any problem with them is an error and not a video without
		// subtitles.
		if !isWebVTT(a.subtitles) {
			webvtt, err := convertToWebVTT(a.subtitles, a.subtitlesTrack)
			if err != nil {
				return nil, err
			}
			a.subtitlesWebVTT = webvtt
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

	if isWebVTT(a.subtitles) {
		http.ServeFile(w, r, a.subtitles)
		return
	}
	w.Write(a.subtitlesWebVTT)
}

// isWebVTT returns whether a subtitles file is already WebVTT.
func isWebVTT(filename string) bool {
	return strings.EqualFold(path.Ext(filename), ".vtt")
}

// convertToWebVTT converts subtitles to WebVTT with ffmpeg. The file can be
// anything ffmpeg can read subtitles from: srt, ass, or even a video with
// embedded subtitles. track is which subtitles of the file to convert,
// starting from 1.
func convertToWebVTT(filename string, track int) ([]byte, error) {
	if track < 1 {
		track = 1
	}

	var stderr bytes.Buffer
	cmd := exec.Command(
		"ffmpeg",
		"-v", "error",
		"-i", filename,
		"-map", fmt.Sprintf("0:s:%d", track-1),
		"-f", "webvtt",
		"pipe:1",
	)
	cmd.Stderr = &stderr
	webvtt, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("unable to convert subtitles %d of %q to WebVTT: %v: %s", track, filename, err, firstLine(stderr.String()))
	}
	return webvtt, nil
}

// firstLine returns the first line of a text.
func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

package application

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const testSrt = `1
00:00:00,000 --> 00:00:02,000
Hello

`

func TestSubtitlesTracks(t *testing.T) {
	// No subtitles, no tracks.
	tracks, err := (&Application{}).subtitlesTracks()
	require.NoError(t, err)
	require.Empty(t, tracks)

	// An url is used as it is.
	a := &Application{}
	WithSubtitles("https://example.com/subtitles.vtt")(a)
	tracks, err = a.subtitlesTracks()
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	require.Equal(t, subtitlesTrackID, tracks[0].TrackId)
	require.Equal(t, "TEXT", tracks[0].Type)
	require.Equal(t, "SUBTITLES", tracks[0].Subtype)
	require.Equal(t, "https://example.com/subtitles.vtt", tracks[0].TrackContentId)
	require.Equal(t, "text/vtt", tracks[0].TrackContentType)

	// A file has to exist.
	a = &Application{}
	WithSubtitles(filepath.Join(t.TempDir(), "missing.srt"))(a)
	_, err = a.subtitlesTracks()
	require.Error(t, err)
}

func TestServeSubtitles(t *testing.T) {
	serve := func(subtitles string) *httptest.ResponseRecorder {
		a := &Application{subtitles: subtitles}
		w := httptest.NewRecorder()
		a.serveSubtitles(w, httptest.NewRequest(http.MethodGet, subtitlesPath, nil))
		return w
	}
	file := func(name, content string) string {
		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}

	t.Run("without subtitles", func(t *testing.T) {
		require.Equal(t, http.StatusNotFound, serve("").Code)
	})

	t.Run("webvtt is sent as it is", func(t *testing.T) {
		w := serve(file("subtitles.vtt", "WEBVTT\n\n00:00.000 --> 00:02.000\nHello\n"))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		require.Equal(t, "text/vtt; charset=utf-8", w.Header().Get("Content-Type"))
		require.Contains(t, w.Body.String(), "Hello")
	})

	t.Run("anything else is converted to webvtt", func(t *testing.T) {
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			t.Skip("ffmpeg is not installed")
		}
		w := serve(file("subtitles.srt", testSrt))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		require.Equal(t, "text/vtt; charset=utf-8", w.Header().Get("Content-Type"))
		require.Contains(t, w.Body.String(), "WEBVTT")
		require.Contains(t, w.Body.String(), "Hello")
	})

	t.Run("fails if it can't be converted", func(t *testing.T) {
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			t.Skip("ffmpeg is not installed")
		}
		w := serve(file("subtitles.srt", "this isn't a subtitles file"))
		require.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

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
	serve := func(a *Application) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.serveSubtitles(w, httptest.NewRequest(http.MethodGet, subtitlesPath, nil))
		return w
	}

	t.Run("without subtitles", func(t *testing.T) {
		require.Equal(t, http.StatusNotFound, serve(&Application{}).Code)
	})

	t.Run("webvtt is sent as it is", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subtitles.vtt")
		require.NoError(t, os.WriteFile(path, []byte("WEBVTT\n\n00:00.000 --> 00:02.000\nHello\n"), 0o600))

		w := serve(&Application{subtitles: path})
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		require.Equal(t, "text/vtt; charset=utf-8", w.Header().Get("Content-Type"))
		require.Contains(t, w.Body.String(), "Hello")
	})

	t.Run("anything else is sent converted", func(t *testing.T) {
		w := serve(&Application{subtitles: "subtitles.srt", subtitlesWebVTT: []byte("WEBVTT\n\nconverted")})
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		require.Equal(t, "text/vtt; charset=utf-8", w.Header().Get("Content-Type"))
		require.Contains(t, w.Body.String(), "converted")
	})
}

func TestConvertToWebVTT(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	file := func(name, content string) string {
		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}
	srt := file("subtitles.srt", testSrt)

	webvtt, err := convertToWebVTT(srt, 1)
	require.NoError(t, err)
	require.Contains(t, string(webvtt), "WEBVTT")
	require.Contains(t, string(webvtt), "Hello")

	// The first subtitles are the default.
	webvtt, err = convertToWebVTT(srt, 0)
	require.NoError(t, err)
	require.Contains(t, string(webvtt), "Hello")

	// There is only one subtitles track in that file.
	_, err = convertToWebVTT(srt, 2)
	require.Error(t, err)

	_, err = convertToWebVTT(file("broken.srt", "this isn't a subtitles file"), 1)
	require.Error(t, err)
}

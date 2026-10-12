package application

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServeStdin(t *testing.T) {
	a := &Application{stdin: strings.NewReader("some media")}

	// Without transcoding, stdin is sent as it is.
	w := httptest.NewRecorder()
	a.serveStdin(w, httptest.NewRequest(http.MethodGet, "/?media_file=-", nil), false)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "some media", w.Body.String())

	// Stdin can't be read again, so it is only served once.
	w = httptest.NewRecorder()
	a.serveStdin(w, httptest.NewRequest(http.MethodGet, "/?media_file=-", nil), false)
	require.Equal(t, http.StatusGone, w.Code)
}

func TestLoadAndServeStdinNeedsContentTypeOrTranscode(t *testing.T) {
	a := &Application{}

	_, err := a.loadAndServeStdin("", false)
	require.Error(t, err)
	require.Empty(t, a.mediaFilenames)
}

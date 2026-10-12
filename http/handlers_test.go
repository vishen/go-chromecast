package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishen/go-chromecast/application/mocks"
)

// A request for a device whose status can't be updated (ie: the connection
// to it is gone) has to fail, and not to return an empty 200.
func TestRequestFailsWhenDeviceCantBeUpdated(t *testing.T) {
	app := &mocks.App{}
	app.On("Update").Return(errors.New("connection to the chromecast lost"))

	h := NewHandler(false)
	h.apps["device-1"] = app

	for _, path := range []string{"/status", "/pause", "/unpause", "/stop", "/volume?volume=0.5"} {
		method := http.MethodPost
		if path == "/status" {
			method = http.MethodGet
		}
		separator := "?"
		if path == "/volume?volume=0.5" {
			separator = "&"
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path+separator+"uuid=device-1", nil))

		require.Equal(t, http.StatusInternalServerError, w.Code, path)
		require.Contains(t, w.Body.String(), "connection to the chromecast lost", path)
	}
}

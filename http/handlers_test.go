package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishen/go-chromecast/application"
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

// request sends a request to the handler and returns the response.
func request(h *Handler, method, url string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, url, nil))
	return w
}

func TestReconnectsWhenDeviceCantBeUpdated(t *testing.T) {
	broken := &mocks.App{}
	broken.On("Update").Return(errors.New("connection to the chromecast lost"))
	broken.On("Close", false).Return(nil)

	working := &mocks.App{}
	working.On("Update").Return(nil)
	working.On("Pause").Return(nil)

	h := NewHandler(false)
	h.addApp("device-1", broken, target{addr: "192.168.0.10", port: 8009, name: "tv"})
	connections := 0
	h.connectFunc = func(addr string, port int, name string) (application.App, error) {
		connections++
		require.Equal(t, "192.168.0.10", addr)
		require.Equal(t, 8009, port)
		require.Equal(t, "tv", name)
		return working, nil
	}

	w := request(h, http.MethodPost, "/pause?uuid=device-1")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, connections)
	broken.AssertCalled(t, "Close", false)
	working.AssertCalled(t, "Pause")

	// The new connection is the one that is used from now on.
	w = request(h, http.MethodPost, "/pause?uuid=device-1")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, connections)
}

func TestFailsWhenItCantReconnect(t *testing.T) {
	broken := &mocks.App{}
	broken.On("Update").Return(errors.New("connection to the chromecast lost"))
	broken.On("Close", false).Return(nil)

	h := NewHandler(false)
	h.addApp("device-1", broken, target{addr: "192.168.0.10", port: 8009})
	h.connectFunc = func(addr string, port int, name string) (application.App, error) {
		return nil, errors.New("no route to host")
	}

	w := request(h, http.MethodPost, "/pause?uuid=device-1")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "connection to the chromecast lost")
	require.Contains(t, w.Body.String(), "no route to host")
}

func TestDeviceByAddr(t *testing.T) {
	app := &mocks.App{}
	app.On("Update").Return(nil)
	app.On("Pause").Return(nil)
	app.On("Close", false).Return(nil)

	h := NewHandler(false)
	connections := 0
	h.connectFunc = func(addr string, port int, name string) (application.App, error) {
		connections++
		require.Equal(t, "192.168.0.10", addr)
		require.Equal(t, 8009, port)
		return app, nil
	}

	// A device that isn't connected yet is connected by its address.
	w := request(h, http.MethodPost, "/pause?addr=192.168.0.10")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, connections)

	// And it is found by its address afterwards.
	w = request(h, http.MethodPost, "/pause?addr=192.168.0.10")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, connections)
	app.AssertNumberOfCalls(t, "Pause", 2)

	w = request(h, http.MethodPost, "/disconnect?addr=192.168.0.10")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Empty(t, h.ConnectedDeviceUUIDs())

	// Connecting explicitly with just the address works too.
	w = request(h, http.MethodPost, "/connect?addr=192.168.0.10")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, []string{"192.168.0.10"}, h.ConnectedDeviceUUIDs())

	// A device connected with a uuid is also found by its address.
	h.addApp("device-2", app, target{addr: "192.168.0.11", port: 8009})
	w = request(h, http.MethodPost, "/pause?addr=192.168.0.11")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 2, connections)

	w = request(h, http.MethodPost, "/pause")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

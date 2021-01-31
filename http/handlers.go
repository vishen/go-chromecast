package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/vishen/go-chromecast/application"
	"github.com/vishen/go-chromecast/dns"
)

type Handler struct {
	mu   sync.Mutex
	apps map[string]*application.Application

	verbose bool
}

func NewHandler(verbose bool) *Handler {
	return &Handler{
		verbose: verbose,
		apps:    map[string]*application.Application{},
		mu:      sync.Mutex{},
	}
}

func (h *Handler) Serve(addr string) error {
	h.logAlways("starting http server on %s", addr)
	h.registerHandlers()
	return http.ListenAndServe(addr, nil)
}

func (h *Handler) registerHandlers() {
	/*
		GET /devices
		POST /connect?uuid=<device_uuid>&addr=<device_addr>&port=<device_port>
		POST /disconnect?uuid=<device_uuid>
		POST /disconnect-all
		POST /status?uuid=<device_uuid>
		POST /pause?uuid=<device_uuid>
		POST /unpause?uuid=<device_uuid>
		POST /mute?uuid=<device_uuid>
		POST /unmute?uuid=<device_uuid>
		POST /stop?uuid=<device_uuid>
		GET /volume?uuid=<device_uuid>
		POST /volume?uuid=<device_uuid>&volume=<float>
		POST /rewind?uuid=<device_uuid>&seconds=<int>
		POST /seek?uuid=<device_uuid>&seconds=<int>
		POST /seek-to?uuid=<device_uuid>&seconds=<float>
		POST /load?uuid=<device_uuid>&path=<filepath_or_url>&content_type=<string>
	*/

	http.HandleFunc("/devices", h.listDevices)
	http.HandleFunc("/connect", h.connect)
	http.HandleFunc("/disconnect", h.disconnect)
	http.HandleFunc("/disconnect-all", h.disconnectAll)
	http.HandleFunc("/status", h.status)
	http.HandleFunc("/pause", h.pause)
	http.HandleFunc("/unpause", h.unpause)
	http.HandleFunc("/mute", h.mute)
	http.HandleFunc("/unmute", h.unmute)
	http.HandleFunc("/stop", h.stop)
	http.HandleFunc("/volume", h.volume)
	http.HandleFunc("/rewind", h.rewind)
	http.HandleFunc("/seek", h.seek)
	http.HandleFunc("/seek-to", h.seekTo)
	http.HandleFunc("/load", h.load)
}

func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	h.log("listing chromecast devices")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	// TODO: Get iface from query params
	devicesChan, err := dns.DiscoverCastDNSEntries(ctx, nil)
	if err != nil {
		h.log("error discovering entries: %v", err)
		httpError(w, fmt.Errorf("unable to discover cast dns entries: %v", err))
		return
	}

	devices := []device{}
	for d := range devicesChan {
		devices = append(devices, device{
			Addr:       d.AddrV4.String(),
			Port:       d.Port,
			Name:       d.Name,
			Host:       d.Host,
			UUID:       d.UUID,
			Device:     d.Device,
			Status:     d.Status,
			DeviceName: d.DeviceName,
			InfoFields: d.InfoFields,
		})
	}

	h.log("found %d devices", len(devices))

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(devices); err != nil {
		h.log("error encoding json: %v", err)
		httpError(w, fmt.Errorf("unable to json encode devices: %v", err))
		return
	}

}

func (h *Handler) app(uuid string) (*application.Application, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	app, ok := h.apps[uuid]
	return app, ok
}

func (h *Handler) connect(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	deviceUUID := q.Get("uuid")
	if deviceUUID == "" {
		httpValidationError(w, "missing 'uuid' in query paramater")
		return
	}

	_, ok := h.app(deviceUUID)
	if ok {
		httpValidationError(w, "device uuid is already connected")
		return
	}

	deviceAddr := q.Get("addr")
	devicePort := q.Get("port")

	if deviceAddr == "" || devicePort == "" {
		h.log("device addr and/or port are missing, trying to lookup address for uuid %q", deviceUUID)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		devicesChan, err := dns.DiscoverCastDNSEntries(ctx, nil)
		if err != nil {
			h.log("error discovering entries: %v", err)
			httpError(w, fmt.Errorf("unable to discover cast dns entries: %v", err))
			return
		}

		for device := range devicesChan {
			// TODO: Should there be a lookup by name as well?
			if device.UUID == deviceUUID {
				deviceAddr = device.AddrV4.String()
				// TODO: This is an unnessecary conversion since
				// we cast back to int a bit later.
				devicePort = strconv.Itoa(device.Port)
			}
		}
	}

	if deviceAddr == "" || devicePort == "" {
		httpValidationError(w, "'port' and 'addr' missing from query params and uuid device lookup returned no results")
		return
	}

	h.log("connecting to addr=%s port=%s...", deviceAddr, devicePort)

	devicePortI, err := strconv.Atoi(devicePort)
	if err != nil {
		h.log("device port %q is not a number: %v", devicePort, err)
		httpValidationError(w, "'port' is not a number")
		return
	}

	applicationOptions := []application.ApplicationOption{
		application.WithDebug(h.verbose),
		application.WithCacheDisabled(true),
	}

	app := application.NewApplication(applicationOptions...)
	if err := app.Start(deviceAddr, devicePortI); err != nil {
		h.log("unable to start application: %v", err)
		httpError(w, fmt.Errorf("unable to start application: %v", err))
		return
	}
	h.mu.Lock()
	h.apps[deviceUUID] = app
	h.mu.Unlock()

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(connectResponse{DeviceUUID: deviceUUID}); err != nil {
		h.log("error encoding json: %v", err)
		httpError(w, fmt.Errorf("unable to json encode devices: %v", err))
		return
	}

}

func (h *Handler) disconnect(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	deviceUUID := q.Get("uuid")
	if deviceUUID == "" {
		httpValidationError(w, "missing 'uuid' in query paramater")
		return
	}

	h.log("disconnecting device %s", deviceUUID)

	app, ok := h.app(deviceUUID)
	if !ok {
		httpValidationError(w, "device uuid is not connected")
		return
	}

	// TODO: Stop media should be a url params? Update
	// disconnectAll appropriately
	stopMedia := false
	if err := app.Close(stopMedia); err != nil {
		h.log("unable to close application: %v", err)
		httpError(w, fmt.Errorf("unable to disconnect from device: %w", err))
		return
	}

	h.mu.Lock()
	delete(h.apps, deviceUUID)
	h.mu.Unlock()
}

func (h *Handler) disconnectAll(w http.ResponseWriter, r *http.Request) {
	h.log("disconnecting all devices")
	h.mu.Lock()
	for deviceUUID, app := range h.apps {
		if err := app.Close(false); err != nil {
			h.log("unable to close application %q: %v", deviceUUID, err)
		}
		delete(h.apps, deviceUUID)
	}
	h.mu.Unlock()
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}
	h.log("status for device")

	castApplication, castMedia, castVolume := app.Status()

	statusResponse := fromApplicationStatus(
		castApplication,
		castMedia,
		castVolume,
	)

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(statusResponse); err != nil {
		h.log("error encoding json: %v", err)
		httpError(w, fmt.Errorf("unable to json encode devices: %v", err))
		return
	}
}

func (h *Handler) pause(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}
	h.log("pausing device")

	if err := app.Pause(); err != nil {
		h.log("unable to pause device: %v", err)
		httpError(w, fmt.Errorf("unable to pause device: %w", err))
		return
	}
}

func (h *Handler) unpause(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}
	h.log("unpausing device")

	if err := app.Unpause(); err != nil {
		h.log("unable to unpause device: %v", err)
		httpError(w, fmt.Errorf("unable to unpause device: %w", err))
		return
	}
}

func (h *Handler) mute(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}
	h.log("muting device")

	if err := app.SetMuted(true); err != nil {
		h.log("unable to mute device: %v", err)
		httpError(w, fmt.Errorf("unable to mute device: %w", err))
		return
	}
}

func (h *Handler) unmute(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}
	h.log("unmuting device")

	if err := app.SetMuted(false); err != nil {
		h.log("unable to unmute device: %v", err)
		httpError(w, fmt.Errorf("unable to unmute device: %w", err))
		return
	}
}

func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}
	h.log("stopping device")

	if err := app.Stop(); err != nil {
		h.log("unable to stop device: %v", err)
		httpError(w, fmt.Errorf("unable to stop device: %w", err))
		return
	}
}

func (h *Handler) volume(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}

	if r.Method == "GET" {
		h.log("getting volume for device")
		_, _, volume := app.Status()

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(volumeResponse{Level: volume.Level, Muted: volume.Muted}); err != nil {
			h.log("error encoding json: %v", err)
			httpError(w, fmt.Errorf("unable to json encode devices: %v", err))
		}
		return
	}

	h.log("setting volume for device")

	q := r.URL.Query()
	volume := q.Get("volume")
	if volume == "" {
		httpValidationError(w, "missing 'volume' in query paramater")
		return
	}

	value, err := strconv.ParseFloat(volume, 32)
	if err != nil {
		h.log("volume %q is not a number: %v", volume, err)
		httpValidationError(w, "'volume' is not a number")
		return
	}

	if err := app.SetVolume(float32(value)); err != nil {
		h.log("unable to set device volume: %v", err)
		httpError(w, fmt.Errorf("unable to set device volume: %w", err))
		return
	}
}

func (h *Handler) rewind(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}

	h.log("rewinding device")

	q := r.URL.Query()
	seconds := q.Get("seconds")
	if seconds == "" {
		httpValidationError(w, "missing 'seconds' in query paramater")
		return
	}

	value, err := strconv.Atoi(seconds)
	if err != nil {
		h.log("seconds %q is not a number: %v", seconds, err)
		httpValidationError(w, "'seconds' is not a number")
		return
	}

	if err := app.Seek(-value); err != nil {
		h.log("unable to rewind device: %v", err)
		httpError(w, fmt.Errorf("unable to rewind device: %w", err))
		return
	}
}

func (h *Handler) seek(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}

	h.log("seeking device")

	q := r.URL.Query()
	seconds := q.Get("seconds")
	if seconds == "" {
		httpValidationError(w, "missing 'seconds' in query paramater")
		return
	}

	value, err := strconv.Atoi(seconds)
	if err != nil {
		h.log("seconds %q is not a number: %v", seconds, err)
		httpValidationError(w, "'seconds' is not a number")
		return
	}

	if err := app.Seek(value); err != nil {
		h.log("unable to seek device: %v", err)
		httpError(w, fmt.Errorf("unable to seek device: %w", err))
		return
	}
}

func (h *Handler) seekTo(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}

	h.log("seeking-to device")

	q := r.URL.Query()
	seconds := q.Get("seconds")
	if seconds == "" {
		httpValidationError(w, "missing 'seconds' in query paramater")
		return
	}

	value, err := strconv.ParseFloat(seconds, 32)
	if err != nil {
		h.log("seconds %q is not a number: %v", seconds, err)
		httpValidationError(w, "'seconds' is not a number")
		return
	}

	if err := app.SeekToTime(float32(value)); err != nil {
		h.log("unable to seek-to device: %v", err)
		httpError(w, fmt.Errorf("unable to seek-to device: %w", err))
		return
	}
}

func (h *Handler) load(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		return
	}

	h.log("loading media for device")

	q := r.URL.Query()
	path := q.Get("path")
	if path == "" {
		httpValidationError(w, "missing 'path' in query paramater")
		return
	}

	contentType := q.Get("content_type")

	if err := app.Load(path, contentType, true, true, true); err != nil {
		h.log("unable to load media for device: %v", err)
		httpError(w, fmt.Errorf("unable to load media for device: %w", err))
		return
	}
}

func (h *Handler) appForRequest(w http.ResponseWriter, r *http.Request) (*application.Application, bool) {
	q := r.URL.Query()

	deviceUUID := q.Get("uuid")
	if deviceUUID == "" {
		httpValidationError(w, "missing 'uuid' in query params")
		return nil, false
	}

	app, ok := h.app(deviceUUID)
	if !ok {
		httpValidationError(w, "device uuid is not connected")
		return nil, false
	}

	app.Update()

	return app, true
}

func (h *Handler) log(msg string, args ...interface{}) {
	if h.verbose {
		log.Printf(fmt.Sprintf(msg, args...))
	}
}

func (h *Handler) logAlways(msg string, args ...interface{}) {
	log.Printf(fmt.Sprintf(msg, args...))
}

func httpError(w http.ResponseWriter, err error) {
	w.Header().Add("Content-Type", "text/plain")
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func httpValidationError(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

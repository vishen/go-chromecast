package http

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vishen/go-chromecast/application"
	"github.com/vishen/go-chromecast/dns"
)

type Handler struct {
	mu   sync.Mutex
	apps map[string]application.App
	mux  *http.ServeMux

	verbose bool

	autoconnectPeriod time.Duration
	autoconnectTicker *time.Ticker

	// autoupdatePeriodSec defines how frequently app.Update method is called in the background.
	autoupdatePeriod time.Duration
	autoupdateTicker *time.Ticker
}

func NewHandler(verbose bool) *Handler {
	handler := &Handler{
		verbose: verbose,
		apps:    map[string]application.App{},
		mux:     http.NewServeMux(),
		mu:      sync.Mutex{},

		autoconnectPeriod: time.Duration(-1),
		autoconnectTicker: nil,

		autoupdatePeriod: time.Duration(-1),
		autoupdateTicker: nil,
	}
	handler.registerHandlers()
	return handler
}

// AutoConnect configures the handler to perform periodic auto-discovery of all the cast devices & groups.
// It's intended to be called just after `NewHandler()`, before the handler is registered in the server.
func (h *Handler) AutoConnect(period time.Duration) error {
	// Setting the autoconnect property - to allow (in future) periodic refresh of the connections.
	h.autoconnectPeriod = period
	if err := h.connectAllInternal("", "3"); err != nil {
		return err
	}
	if h.autoconnectPeriod > 0 {
		h.autoconnectTicker = time.NewTicker(period)
		go func() {
			for {
				<-h.autoconnectTicker.C
				if err := h.connectAllInternal("", "3"); err != nil {
					log.Printf("AutoConnect issued connectAllInternal failed: %v", err)
				}
			}
		}()
	}
	return nil
}

// AutoUpdate configures the handler to perform auto-update of all the cast devices & groups.
// It's intended to be called just after `NewHandler()`, before the handler is registered in the server.
// Thanks to AutoUpdate, /status and /statuses returns relatively recent status 'instantly'.
func (h *Handler) AutoUpdate(period time.Duration) error {
	// Setting the autoconnect property - to allow (in future) periodic refresh of the connections.
	h.autoupdatePeriod = period
	if h.autoupdatePeriod > 0 {
		h.autoupdateTicker = time.NewTicker(period)
		go func() {
			for {
				<-h.autoupdateTicker.C
				if err := h.UpdateAll(); err != nil {
					log.Printf("AutoUpdate issued UpdateAll failed: %v", err)
				}
			}
		}()
	}
	return nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) Serve(addr string) error {
	log.Printf("starting http server on %s", addr)
	return http.ListenAndServe(addr, h)
}

func (h *Handler) registerHandlers() {
	/*
		GET /devices?wait=...&iface=...
		POST /connect?uuid=<device_uuid>&addr=<device_addr>&port=<device_port>
		POST /connect-all?wait=...&iface=...
		POST /disconnect?uuid=<device_uuid>
		POST /disconnect-all
		GET /status?uuid=<device_uuid>
		GET /statuses
		POST /pause?uuid=<device_uuid>
		POST /unpause?uuid=<device_uuid>
		POST /skipad?uuid=<device_uuid>
		POST /mute?uuid=<device_uuid>
		POST /unmute?uuid=<device_uuid>
		POST /stop?uuid=<device_uuid>
		GET /volume?uuid=<device_uuid>
		POST /volume?uuid=<device_uuid>&volume=<float>
		POST /rewind?uuid=<device_uuid>&seconds=<int>
		POST /seek?uuid=<device_uuid>&seconds=<int>
		POST /seek-to?uuid=<device_uuid>&seconds=<float>
		POST /load?uuid=<device_uuid>&path=<filepath_or_url>&content_type=<string>&start_time=<int>
	*/

	h.mux.HandleFunc("/devices", h.listDevices)
	h.mux.HandleFunc("/connect", h.connect)
	h.mux.HandleFunc("/connect-all", h.connectAll)
	h.mux.HandleFunc("/disconnect", h.disconnect)
	h.mux.HandleFunc("/disconnect-all", h.disconnectAll)
	h.mux.HandleFunc("/status", h.status)
	h.mux.HandleFunc("/statuses", h.statuses)
	h.mux.HandleFunc("/pause", h.pause)
	h.mux.HandleFunc("/unpause", h.unpause)
	h.mux.HandleFunc("/skipad", h.skipad)
	h.mux.HandleFunc("/mute", h.mute)
	h.mux.HandleFunc("/unmute", h.unmute)
	h.mux.HandleFunc("/stop", h.stop)
	h.mux.HandleFunc("/volume", h.volume)
	h.mux.HandleFunc("/rewind", h.rewind)
	h.mux.HandleFunc("/seek", h.seek)
	h.mux.HandleFunc("/seek-to", h.seekTo)
	h.mux.HandleFunc("/load", h.load)
}

func (h *Handler) discoverDnsEntries(ctx context.Context, iface string, waitq string) (devices []device) {
	wait := 3
	if n, err := strconv.Atoi(waitq); err == nil {
		wait = n
	}

	devices = []device{}
	var interf *net.Interface
	if iface != "" {
		var err error
		interf, err = net.InterfaceByName(iface)
		if err != nil {
			h.log("error discovering entries: %v", err)
			return
		}
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(wait)*time.Second)
	defer cancel()

	devicesChan, err := dns.DiscoverCastDNSEntries(ctx, interf)
	if err != nil {
		h.log("error discovering entries: %v", err)
		return
	}

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

	return
}

func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	h.log("listing chromecast devices")

	q := r.URL.Query()
	iface := q.Get("interface")
	wait := q.Get("wait")

	devices := h.discoverDnsEntries(context.Background(), iface, wait)
	h.log("found %d devices", len(devices))

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(devices); err != nil {
		h.log("error encoding json: %v", err)
		httpError(w, fmt.Errorf("unable to json encode devices: %v", err))
		return
	}

}

func (h *Handler) app(uuid string) (application.App, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	app, ok := h.apps[uuid]
	return app, ok
}

func (h *Handler) ConnectedDeviceUUIDs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	var uuids []string
	for k, _ := range h.apps {
		uuids = append(uuids, k)
	}
	return uuids
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
	deviceName := q.Get("name")
	iface := q.Get("interface")
	wait := q.Get("wait")

	if deviceAddr == "" || devicePort == "" || (deviceName == "" && devicePort != "8009") {
		h.log("device addr and/or port are missing, trying to lookup address for uuid %q", deviceUUID)

		devices := h.discoverDnsEntries(context.Background(), iface, wait)
		for _, device := range devices {
			// TODO: Should there be a lookup by name as well?
			if device.UUID == deviceUUID {
				deviceAddr = device.Addr
				// TODO: This is an unnessecary conversion since
				// we cast back to int a bit later.
				devicePort = strconv.Itoa(device.Port)
				deviceName = device.DeviceName
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

	app, err := h.connectInternal(deviceAddr, devicePortI, deviceName)
	if err != nil {
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

func (h *Handler) connectInternal(deviceAddr string, devicePort int, deviceName string) (application.App, error) {
	applicationOptions := []application.ApplicationOption{
		application.WithDebug(h.verbose),
		application.WithCacheDisabled(true),
	}
	if deviceName != "" {
		applicationOptions = append(applicationOptions, application.WithDeviceNameOverride(deviceName))
	}

	app := application.NewApplication(applicationOptions...)
	if err := app.Start(deviceAddr, devicePort); err != nil {
		return nil, err
	}
	return app, nil
}

func (h *Handler) connectAll(w http.ResponseWriter, r *http.Request) {
	h.log("connecting all devices")
	q := r.URL.Query()

	wait := q.Get("wait")
	iface := q.Get("interface")

	err := h.connectAllInternal(iface, wait)
	if err != nil {
		h.log("error connecting: %v", err)
		httpError(w, fmt.Errorf("unable to connect to device: %v", err))
	}

	var resp []connectResponse
	for _, deviceUUID := range h.ConnectedDeviceUUIDs() {
		resp = append(resp, connectResponse{DeviceUUID: deviceUUID})
	}

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log("error encoding json: %v", err)
		httpError(w, fmt.Errorf("unable to json encode devices: %v", err))
		return
	}
}

func (h *Handler) connectAllInternal(iface string, waitSec string) error {
	ctx := context.Background()
	devices := h.discoverDnsEntries(ctx, iface, waitSec)
	uuidMap := map[string]application.App{}
	g, ctx := errgroup.WithContext(ctx)
	for _, device := range devices {
		g.Go(func() error {
			app, err := h.connectInternal(device.Addr, device.Port, device.DeviceName)
			if err != nil {
				return err
			}
			uuidMap[device.UUID] = app
			return nil
		})
	}
	err := g.Wait()
	h.mu.Lock()
	// Even if we cannot connect to some of the devices, we still update the map for remaining devices.
	h.apps = uuidMap
	h.mu.Unlock()
	return err
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

	stopMedia := q.Get("stop") == "true"
	if err := app.Close(stopMedia); err != nil {
		h.log("unable to close application: %v", err)
	}

	h.mu.Lock()
	delete(h.apps, deviceUUID)
	h.mu.Unlock()
}

func (h *Handler) disconnectAll(w http.ResponseWriter, r *http.Request) {
	h.log("disconnecting all devices")
	h.mu.Lock()
	stopMedia := r.URL.Query().Get("stop") == "true"
	for deviceUUID, app := range h.apps {
		if err := app.Close(stopMedia); err != nil {
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
	syncUpdate := r.URL.Query().Get("syncUpdate") == "true"
	if syncUpdate {
		if err := app.Update(); err != nil {
			h.log("error updating  status: %v", err)
			httpError(w, fmt.Errorf("error updating status: %w", err))
			return
		}
	}
	castApplication, castMedia, castVolume := app.Status()
	info, err := app.Info()
	if err != nil {
		werr := fmt.Errorf("error getting device info: %v", err)
		h.log("%v", werr)
		httpError(w, werr)
		return
	}
	statusResponse := fromApplicationStatus(
		info,
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

func (h *Handler) UpdateAll() error {
	uuids := h.ConnectedDeviceUUIDs()
	g := new(errgroup.Group)
	for _, deviceUUID := range uuids {
		app, ok := h.app(deviceUUID)
		if ok {
			g.Go(func() error { return app.Update() })
		}
	}
	return g.Wait()
}

func (h *Handler) statuses(w http.ResponseWriter, r *http.Request) {
	h.log("statuses for devices")
	syncUpdate := r.URL.Query().Get("syncUpdate") == "true"
	uuids := h.ConnectedDeviceUUIDs()
	mapUUID2Ch := map[string]chan statusResponse{}
	g := new(errgroup.Group)
	for _, deviceUUID := range uuids {
		app, ok := h.app(deviceUUID)
		if ok {
			ch := make(chan statusResponse, 1)
			mapUUID2Ch[deviceUUID] = ch
			g.Go(func() error {
				if syncUpdate {
					if err := app.Update(); err != nil {
						return err
					}
				}
				castApplication, castMedia, castVolume := app.Status()
				info, err := app.Info()
				if err != nil {
					return fmt.Errorf("error getting device info: %v", err)
				}
				ch <- fromApplicationStatus(
					info,
					castApplication,
					castMedia,
					castVolume,
				)
				return nil
			})
		}
	}
	if err := g.Wait(); err != nil {
		h.log("collecting statuses failed: %v", err)
		httpError(w, fmt.Errorf("collecting statuses failed: %w", err))
		return
	}

	statusResponses := map[string]statusResponse{}
	for deviceUUID, ch := range mapUUID2Ch {
		statusResponse := <-ch
		statusResponses[deviceUUID] = statusResponse
	}

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(statusResponses); err != nil {
		h.log("error encoding json: %v", err)
		httpError(w, fmt.Errorf("unable to json encode statuses: %v", err))
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

func (h *Handler) skipad(w http.ResponseWriter, r *http.Request) {
	app, found := h.appForRequest(w, r)
	if !found {
		h.log("handlers.go.skipad: !found for skipad")
		return
	}

	h.log("skipping ad")

	if err := app.Skipad(); err != nil {
		h.log("unable to skip ad for: %v", err)
		httpError(w, fmt.Errorf("unable to skip ad for: %w", err))
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
		httpValidationError(w, "missing 'seconds' in query parameter")
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
		httpValidationError(w, "missing 'seconds' in query parameter")
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

	startTime := q.Get("start_time")
	if startTime == "" {
		startTime = "0"
	}

	startTimeInt, err := strconv.Atoi(startTime)

	if err != nil {
		h.log("start_time %q is not a number: %v", startTimeInt, err)
		httpValidationError(w, "'start_time' is not a number")
		return
	}

	if err := app.Load(path, startTimeInt, contentType, true, true, true); err != nil {
		h.log("unable to load media for device: %v", err)
		httpError(w, fmt.Errorf("unable to load media for device: %w", err))
		return
	}
}

func (h *Handler) appForRequest(w http.ResponseWriter, r *http.Request) (application.App, bool) {
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

	if err := app.Update(); err != nil {
		return nil, false
	}

	return app, true
}

func (h *Handler) log(msg string, args ...interface{}) {
	if h.verbose {
		log.Printf(msg, args...)
	}
}

func httpError(w http.ResponseWriter, err error) {
	w.Header().Add("Content-Type", "text/plain")
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func httpValidationError(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

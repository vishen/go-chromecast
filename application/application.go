package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/buger/jsonparser"
	"github.com/pkg/errors"

	"github.com/vishen/go-chromecast/cast"
	pb "github.com/vishen/go-chromecast/cast/proto"
	castdns "github.com/vishen/go-chromecast/dns"
	"github.com/vishen/go-chromecast/storage"
)

var (
	// Global request id
	requestID int
)

const (
	// 'CC1AD845' seems to be a predefined app; check link
	// https://gist.github.com/jloutsenhizer/8855258
	// https://github.com/thibauts/node-castv2
	defaultChromecastAppId = "CC1AD845"

	defaultSender = "sender-0"
	defaultRecv   = "receiver-0"

	namespaceConn  = "urn:x-cast:com.google.cast.tp.connection"
	namespaceRecv  = "urn:x-cast:com.google.cast.receiver"
	namespaceMedia = "urn:x-cast:com.google.cast.media"
)

type PlayedItem struct {
	ContentID string `json:"content_id"`
	Started   int64  `json:"started"`
	Finished  int64  `json:"finished"`
}

type CastMessageFunc func(*pb.CastMessage)

type Application struct {
	conn  *cast.Connection
	debug bool

	// 'cast.Connection' will send receieved messages back on this channel.
	recvMsgChan chan *pb.CastMessage
	// Internal mapping of request id to result channel
	resultChanMap map[int]chan *pb.CastMessage

	messageMu sync.Mutex
	// Relay messages receieved so users can add custom logic to
	// events.
	messageChan chan *pb.CastMessage
	// Functions that will receieve messages from 'messageChan'
	messageFuncs []CastMessageFunc

	// Current values from the chromecast.
	application *cast.Application // It is possible that there is no current application, can happen for goole home.
	media       *cast.Media
	// There seems to be two different volumes returned from the chromecast,
	// one for the receiever and one for the playing media. It looks we update
	// the receiever volume from go-chromecast so we should use that one. But
	// we will keep the other one around in-case we need it at some point.
	volumeMedia    *cast.Volume
	volumeReceiver *cast.Volume

	httpServer *http.Server
	serverPort int
	localIP    string
	iface      string

	// NOTE: Currently only playing one media file at a time is handled
	mediaFinished  chan bool
	mediaFilenames []string

	playedItems   map[string]PlayedItem
	cacheDisabled bool
	cache         *storage.Storage
}

func NewApplication(iface string, debug, cacheDisabled bool) *Application {
	// TODO(vishen): make cast.Connection an interface, most likely will just need
	// the Send method
	// Channel to receive messages from the cast connecttion. 5 is a randomly
	// chosen number.
	recvMsgChan := make(chan *pb.CastMessage, 5)
	a := &Application{
		recvMsgChan:   recvMsgChan,
		resultChanMap: map[int]chan *pb.CastMessage{},
		messageChan:   make(chan *pb.CastMessage),
		conn:          cast.NewConnection(recvMsgChan, debug),
		debug:         debug,
		cacheDisabled: cacheDisabled,
		playedItems:   map[string]PlayedItem{},
		cache:         storage.NewStorage(),
		iface:         iface,
	}
	// Kick off the listener for asynchronous messages received from the
	// cast connection.
	go a.recvMessages()
	// Kick off the message channel listener.
	go a.messageChanHandler()
	return a
}

func (a *Application) AddMessageFunc(f CastMessageFunc) {
	a.messageMu.Lock()
	defer a.messageMu.Unlock()

	a.messageFuncs = append(a.messageFuncs, f)
}

func (a *Application) messageChanHandler() {
	for msg := range a.messageChan {
		a.messageMu.Lock()
		for _, f := range a.messageFuncs {
			f(msg)
		}
		a.messageMu.Unlock()
	}
}

func (a *Application) recvMessages() {
	for msg := range a.recvMsgChan {
		requestID, err := jsonparser.GetInt([]byte(*msg.PayloadUtf8), "requestId")
		if err == nil {
			if resultChan, ok := a.resultChanMap[int(requestID)]; ok {
				resultChan <- msg
				// Relay the event to any user specified message funcs.
				a.messageChan <- msg
				continue
			}
		}

		messageBytes := []byte(*msg.PayloadUtf8)
		// This already gets checked in the cast.Connection.handleMessage function.
		messageType, _ := jsonparser.GetString(messageBytes, "type")
		switch messageType {
		case "LOAD_FAILED":
			a.mediaFinished <- true
		case "MEDIA_STATUS":
			resp := cast.MediaStatusResponse{}
			if err := json.Unmarshal(messageBytes, &resp); err == nil {
				for _, status := range resp.Status {
					// The LoadingItemId is only set when there is a playlist and there
					// is an item being loaded to play next.
					if status.IdleReason == "FINISHED" && status.LoadingItemId == 0 {
						a.mediaFinished <- true
					} else if status.IdleReason == "INTERRUPTED" && status.Media.ContentId == "" {
						// This can happen when we go "next" in a playlist when it
						// is playing the last track.
						a.mediaFinished <- true
					}
				}
			}
		case "RECEIVER_STATUS":
			// We don't care about this when the application isn't set.
			if a.application == nil {
				break
			}
			resp := cast.ReceiverStatusResponse{}
			if err := json.Unmarshal(messageBytes, &resp); err == nil {
				// Check to see if the application on the device has changed,
				// if it has it is likely not this running instance that changed
				// it because that currently isn't possible.
				for _, app := range resp.Status.Applications {
					if app.AppId != a.application.AppId {
						a.mediaFinished <- true
					}
					a.application = &app
				}
				a.volumeReceiver = &resp.Status.Volume
			}
		}
		// Relay the event to any user specified message funcs.
		a.messageChan <- msg
	}
}

func (a *Application) SetDebug(debug bool) { a.debug = debug; a.conn.SetDebug(debug) }

func (a *Application) Start(entry castdns.CastDNSEntry) error {
	if err := a.loadPlayedItems(); err != nil {
		a.log("unable to load played items: %v", err)
	}

	if err := a.conn.Start(entry.GetAddr(), entry.GetPort()); err != nil {
		return err
	}
	if err := a.sendDefaultConn(&cast.ConnectHeader); err != nil {
		return errors.Wrap(err, "unable to connect to chromecast")
	}
	return errors.Wrap(a.Update(), "unable to update application")
}

func (a *Application) loadPlayedItems() error {
	if a.cacheDisabled {
		return nil
	}

	b, err := a.cache.Load("application")
	if err != nil || len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, &a.playedItems)
}

func (a *Application) writePlayedItems() error {
	if a.cacheDisabled {
		return nil
	}

	playedItemsJson, _ := json.Marshal(a.playedItems)
	return a.cache.Save("application", playedItemsJson)
}

func (a *Application) Update() error {
	var recvStatus *cast.ReceiverStatusResponse
	var err error
	// Simple retry. We need this for when the device isn't currently
	// available, but it is likely that it will come up soon.
	for i := 0; i < 5; i++ {
		recvStatus, err = a.getReceiverStatus()
		if err == nil {
			break
		}
		a.log("unable to get status from device; attempt %d/5, retrying...", i+1)
		time.Sleep(time.Second * 2)
	}
	if err != nil {
		return err
	}

	if len(recvStatus.Status.Applications) > 1 {
		a.log("more than 1 connected application on the chromecast: (%d)%#v", len(recvStatus.Status.Applications), recvStatus.Status.Applications)
	}

	// TODO(vishen): Why could there be more than one application, how to handle this?
	// For now just take the last one.
	for _, app := range recvStatus.Status.Applications {
		a.application = &app
	}
	a.volumeReceiver = &recvStatus.Status.Volume

	if a.application == nil || a.application.IsIdleScreen {
		return nil
	}

	a.updateMediaStatus()

	return nil

}

func (a *Application) updateMediaStatus() error {
	a.sendMediaConn(&cast.ConnectHeader)

	mediaStatus, err := a.getMediaStatus()
	if err != nil {
		return err
	}
	for _, media := range mediaStatus.Status {
		a.media = &media
		a.volumeMedia = &media.Volume
	}
	return nil
}

func (a *Application) Close() {
	a.sendMediaConn(&cast.CloseHeader)
	a.sendDefaultConn(&cast.CloseHeader)
}

func (a *Application) Status() (*cast.Application, *cast.Media, *cast.Volume) {
	return a.application, a.media, a.volumeReceiver
}

func (a *Application) Pause() error {
	if a.media == nil {
		return ErrNoMediaPause
	}
	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.PauseHeader,
		MediaSessionId: a.media.MediaSessionId,
	})
}

func (a *Application) Unpause() error {
	if a.media == nil {
		return ErrNoMediaUnpause
	}
	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.PlayHeader,
		MediaSessionId: a.media.MediaSessionId,
	})
}

func (a *Application) StopMedia() error {
	if a.media == nil {
		return ErrNoMediaStop
	}
	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.StopHeader,
		MediaSessionId: a.media.MediaSessionId,
	})
}

func (a *Application) Stop() error {
	return a.sendDefaultRecv(&cast.StopHeader)
}

func (a *Application) Next() error {
	if a.media == nil {
		return ErrNoMediaNext
	}

	// TODO(vishen): Get the number of queue items, if none, possibly just skip to the end?
	return a.sendMediaRecv(&cast.QueueUpdate{
		PayloadHeader:  cast.QueueUpdateHeader,
		MediaSessionId: a.media.MediaSessionId,
		Jump:           1,
	})
}

func (a *Application) Previous() error {
	if a.media == nil {
		return ErrNoMediaPrevious
	}

	// TODO(vishen): Get the number of queue items, if none, possibly just jump to beginning?
	return a.sendMediaRecv(&cast.QueueUpdate{
		PayloadHeader:  cast.QueueUpdateHeader,
		MediaSessionId: a.media.MediaSessionId,
		Jump:           -1,
	})
}

func (a *Application) Skip() error {

	if a.media == nil {
		return ErrNoMediaSkip
	}

	// Get the latest media status
	// TODO(vishen): can we unroll this, so it doesn't update the current state?
	// but just returns it?
	// that might also make a.media == nil checks pointless?
	a.updateMediaStatus()

	v := a.media.CurrentTime - 10
	if a.media.Media.Duration > 0 {
		v = a.media.Media.Duration - 10
	}

	return a.Seek(int(v))
}

func (a *Application) Seek(value int) error {
	if a.media == nil {
		return ErrMediaNotYetInitialised
	}

	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.SeekHeader,
		MediaSessionId: a.media.MediaSessionId,
		RelativeTime:   float32(value),
		ResumeState:    "PLAYBACK_START",
	})
}

func (a *Application) SeekFromStart(value int) error {
	if a.media == nil {
		return ErrMediaNotYetInitialised
	}

	// Get the latest media status
	// TODO(vishen): can we unroll this, so it doesn't update the current state?
	// but just returns it?
	// that might also make a.media == nil checks pointless?
	a.updateMediaStatus()

	// TODO(vishen): maybe there is another ResumeState that lets us
	// seek from the end? Although not sure how this works for live media?

	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.SeekHeader,
		MediaSessionId: a.media.MediaSessionId,
		CurrentTime:    float32(value),
		ResumeState:    "PLAYBACK_START",
	})
}

func (a *Application) SetVolume(value float32) error {
	if value > 1 || value < 0 {
		return ErrVolumeOutOfRange
	}

	return a.sendDefaultRecv(&cast.SetVolume{
		PayloadHeader: cast.VolumeHeader,
		Volume: cast.Volume{
			Level: value,
		},
	})
}

func (a *Application) SetMuted(value bool) error {
	return a.sendDefaultRecv(&cast.SetVolume{
		PayloadHeader: cast.VolumeHeader,
		Volume: cast.Volume{
			Muted: value,
		},
	})
}

func (a *Application) getMediaStatus() (*cast.MediaStatusResponse, error) {
	apiMessage, err := a.sendAndWaitMediaRecv(&cast.GetStatusHeader)
	if err != nil {
		return nil, err
	}
	var response cast.MediaStatusResponse
	if err := json.Unmarshal([]byte(*apiMessage.PayloadUtf8), &response); err != nil {
		return nil, errors.Wrap(err, "error unmarshaling json")
	}
	return &response, nil
}

func (a *Application) getReceiverStatus() (*cast.ReceiverStatusResponse, error) {
	apiMessage, err := a.sendAndWaitDefaultRecv(&cast.GetStatusHeader)
	if err != nil {
		return nil, err
	}
	var response cast.ReceiverStatusResponse
	if err := json.Unmarshal([]byte(*apiMessage.PayloadUtf8), &response); err != nil {
		return nil, errors.Wrap(err, "error unmarshaling json")
	}
	return &response, nil

}

func (a *Application) PlayableMediaType(filename string) bool {
	if a.knownFileType(filename) {
		return true
	}

	switch path.Ext(filename) {
	case ".avi":
		return true
	}

	return false
}

func (a *Application) possibleContentType(filename string) (string, error) {
	// TODO(vishen): Inspect the file for known headers?
	// Currently we just check the file extension

	// Can use the following from the Go std library
	// mime.TypesByExtenstion(filepath.Ext(filename))
	// fs.DetectContentType(data []byte) // needs opened(ish) file

	// https://developers.google.com/cast/docs/media
	switch ext := path.Ext(filename); ext {
	case ".mp4", ".m4a", ".m4p", ".MP4":
		return "video/mp4", nil
	case ".webm":
		return "video/webm", nil
	case ".mp3":
		return "audio/mp3", nil
	case ".flac":
		return "audio/flac", nil
	default:
		return "", fmt.Errorf("unknown file extension %q", ext)
	}
}

func (a *Application) knownFileType(filename string) bool {
	if ct, _ := a.possibleContentType(filename); ct != "" {
		return true
	}
	return false
}

func (a *Application) PlayedItems() map[string]PlayedItem {
	return a.playedItems
}

func (a *Application) Load(filenameOrUrl, contentType string, transcode, detach bool) error {
	var mi mediaItem
	isExternalMedia := false
	if strings.HasPrefix(filenameOrUrl, "http://") || strings.HasPrefix(filenameOrUrl, "https://") {
		isExternalMedia = true
		if contentType == "" {
			var err error
			contentType, err = a.possibleContentType(filenameOrUrl)
			if err != nil {
				return err
			}
		}
		mi = mediaItem{
			contentURL:  filenameOrUrl,
			contentType: contentType,
		}
	} else {
		mediaItems, err := a.loadAndServeFiles([]string{filenameOrUrl}, contentType, transcode)
		if err != nil {
			return errors.Wrap(err, "unable to load and serve files")
		}

		if len(mediaItems) != 1 {
			return fmt.Errorf("was expecting 1 media item, received %d", len(mediaItems))
		}
		mi = mediaItems[0]
	}

	if !isExternalMedia && detach {
		return fmt.Errorf("unable to detach from locally playing media content")
	}

	// If the current chromecast application isn't the Default Media Receiver
	// we need to change it
	if a.application == nil || a.application.AppId != defaultChromecastAppId {
		_, err := a.sendAndWaitDefaultRecv(&cast.LaunchRequest{
			PayloadHeader: cast.LaunchHeader,
			AppId:         defaultChromecastAppId,
		})
		if err != nil {
			return errors.Wrap(err, "unable to change to default media receiver")
		}
		// Update the 'application' and 'media' field on the 'CastApplication'
		a.Update()
	}

	// NOTE: This isn't concurrent safe, but it doesn't need to be at the moment!
	a.mediaFinished = make(chan bool, 1)

	// Send the command to the chromecast
	a.sendMediaRecv(&cast.LoadMediaCommand{
		PayloadHeader: cast.LoadHeader,
		CurrentTime:   0,
		Autoplay:      true,
		Media: cast.MediaItem{
			ContentId:   mi.contentURL,
			StreamType:  "BUFFERED",
			ContentType: mi.contentType,
		},
	})

	// If we should detach from waiting for media to finish playing
	// and this is a url loaded external media, then we can exit early.
	if detach && isExternalMedia {
		return nil
	}

	// Wait until we have been notified that the media has finished playing
	<-a.mediaFinished
	return nil
}

func (a *Application) QueueLoad(filenames []string, contentType string, transcode bool) error {

	mediaItems, err := a.loadAndServeFiles(filenames, contentType, transcode)
	if err != nil {
		return errors.Wrap(err, "unable to load and serve files")
	}

	// If the current chromecast application isn't the Default Media Receiver
	// we need to change it
	if a.application == nil || a.application.AppId != defaultChromecastAppId {
		_, err := a.sendAndWaitDefaultRecv(&cast.LaunchRequest{
			PayloadHeader: cast.LaunchHeader,
			AppId:         defaultChromecastAppId,
		})

		if err != nil {
			return errors.Wrap(err, "unable to change to default media receiver")
		}
	}

	// Update the 'application' and 'media' field on the 'CastApplication'
	a.Update()

	items := make([]cast.QueueLoadItem, len(mediaItems))
	for i, mi := range mediaItems {
		items[i] = cast.QueueLoadItem{
			Autoplay:         true,
			PlaybackDuration: 60,
			Media: cast.MediaItem{
				ContentId:   mi.contentURL,
				StreamType:  "BUFFERED",
				ContentType: mi.contentType,
			},
		}
	}

	// Send the command to the chromecast
	a.sendMediaRecv(&cast.QueueLoad{
		PayloadHeader: cast.QueueLoadHeader,
		CurrentTime:   0,
		StartIndex:    0,
		RepeatMode:    "REPEAT_OFF",
		Items:         items,
	})

	// Wait until we have been notified that the media has finished playing
	<-a.mediaFinished
	return nil
}

type mediaItem struct {
	filename    string
	contentType string
	contentURL  string
	transcode   bool
}

func (a *Application) loadAndServeFiles(filenames []string, contentType string, transcode bool) ([]mediaItem, error) {
	mediaItems := make([]mediaItem, len(filenames))
	for i, filename := range filenames {
		transcodeFile := transcode
		if _, err := os.Stat(filename); err != nil {
			return nil, errors.Wrapf(err, "unable to find %q", filename)
		}
		/*
			We can play media for the following:

			- if we have a filename with a known content type
			- if we have a filename, and a specified contentType
			- if we have a filename with an unknown content type, and transcode is true
			-
		*/
		knownFileType := a.knownFileType(filename)
		if !knownFileType && contentType == "" && !transcodeFile {
			return nil, fmt.Errorf("unknown content-type for %q, either specify a content-type or set transcode to true", filename)
		}

		// If we have a content-type specified we should always
		// attempt to use that
		if contentType != "" {
		} else if knownFileType {
			// If this is a media file we know the chromecast can play,
			// then we don't need to transcode it.
			contentType, _ = a.possibleContentType(filename)
			transcodeFile = false
		} else if transcodeFile {
			contentType = "video/mp4"
		}

		mediaItems[i] = mediaItem{
			filename:    filename,
			contentType: contentType,
			transcode:   transcodeFile,
		}
		fmt.Printf("TRANSCODE_DEBUG: %#v\n", mediaItems[i])
		// Add the filename to the list of filenames that go-chromecast will serve.
		a.mediaFilenames = append(a.mediaFilenames, filename)
	}

	// TODO: maybe cache this somewhere
	localIP, err := a.getLocalIP()
	if err != nil {
		return nil, err
	}

	a.log("starting streaming server...")
	// Start server to serve the media
	if err := a.startStreamingServer(); err != nil {
		return nil, errors.Wrap(err, "unable to start streaming server")
	}
	a.log("started streaming server")

	// We can only set the content url after the server has started, otherwise we have
	// no way to know the port used.
	for i, m := range mediaItems {
		mediaItems[i].contentURL = fmt.Sprintf("http://%s:%d?media_file=%s&live_streaming=%t", localIP, a.serverPort, m.filename, m.transcode)
	}

	return mediaItems, nil
}

func (a *Application) getLocalIP() (string, error) {
	if a.localIP != "" {
		return a.localIP, nil
	}

	var addrs []net.Addr
	if a.iface == "" {
		var err error
		addrs, err = net.InterfaceAddrs()
		if err != nil {
			return "", err
		}
	} else {
		ifs, err := net.Interfaces()
		if err != nil {
			return "", err
		}

		var foundIface net.Interface
		for _, i := range ifs {
			if i.Name == a.iface {
				foundIface = i
				break
			}
		}
		if foundIface.Name == "" {
			return "", fmt.Errorf("no network interface with name %q exists", a.iface)
		}
		addrs, err = foundIface.Addrs()
		if err != nil {
			return "", err
		}
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			// TODO(vishen): Fallback to ipv6 if ipv4 fails? Or maybe do the other
			// way around? I am unsure if chromecast supports ipv6???
			if ipnet.IP.To4() != nil {
				a.localIP = ipnet.IP.String()
				return a.localIP, nil
			}
		}
	}
	return "", fmt.Errorf("Failed to get local ip address")
}

func (a *Application) startStreamingServer() error {
	if a.httpServer != nil {
		return nil
	}
	a.log("trying to find available port to start streaming server on")

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return errors.Wrap(err, "unable to bind to local tcp address")
	}

	a.serverPort = listener.Addr().(*net.TCPAddr).Port
	a.log("found available port :%d", a.serverPort)

	a.httpServer = &http.Server{}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check to see if we have a 'filename' and if it is one of the ones that have
		// already been validated and is useable.
		filename := r.URL.Query().Get("media_file")
		canServe := false
		for _, fn := range a.mediaFilenames {
			if fn == filename {
				canServe = true
			}
		}

		a.playedItems[filename] = PlayedItem{ContentID: filename, Started: time.Now().Unix()}
		a.writePlayedItems()

		// Check to see if this is a live streaming video and we need to use an
		// infinite range request / response. This comes from media that is either
		// live or currently being transcoded to a different media format.
		liveStreaming := false
		if ls := r.URL.Query().Get("live_streaming"); ls == "true" {
			liveStreaming = true
		}

		a.log("canServe=%t, liveStreaming=%t, filename=%s", canServe, liveStreaming, filename)
		if canServe {
			if !liveStreaming {
				http.ServeFile(w, r, filename)
			} else {
				a.serveLiveStreaming(w, r, filename)
			}
		} else {
			http.Error(w, "Invalid file", 400)
		}
		a.log("method=%s, headers=%v, reponse_headers=%v", r.Method, r.Header, w.Header())
		pi := a.playedItems[filename]

		// TODO(vishen): make this a pointer?
		pi.Finished = time.Now().Unix()
		a.playedItems[filename] = pi
		a.writePlayedItems()
	})

	go func() {
		a.log("media server listening on %d", a.serverPort)
		if err := a.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.WithField("package", "application").WithError(err).Fatal("error serving HTTP")
		}
	}()

	return nil
}

func (a *Application) serveLiveStreaming(w http.ResponseWriter, r *http.Request, filename string) {
	cmd := exec.Command(
		"ffmpeg",
		"-re", // encode at 1x playback speed, to not burn the CPU
		"-i", filename,
		"-vcodec", "h264",
		"-acodec", "aac",
		"-ac", "2", // chromecasts don't support more than two audio channels
		"-f", "mp4",
		"-movflags", "frag_keyframe+faststart",
		"-strict", "-experimental",
		"pipe:1",
	)

	cmd.Stdout = w
	if a.debug {
		cmd.Stderr = os.Stderr
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")

	if err := cmd.Run(); err != nil {
		log.WithField("package", "application").WithFields(logrus.Fields{
			"filename": filename,
		}).WithError(err).Error("error transcoding")
	}
}

func (a *Application) log(message string, args ...interface{}) {
	if a.debug {
		log.WithField("package", "application").Infof(message, args...)
	}
}

func (a *Application) send(payload cast.Payload, sourceID, destinationID, namespace string) (int, error) {
	// NOTE: Not concurrent safe, but currently only synchronous flow is possible
	// TODO(vishen): just make concurrent safe regardless of current flow
	requestID += 1
	payload.SetRequestId(requestID)
	return requestID, a.conn.Send(requestID, payload, sourceID, destinationID, namespace)
}

func (a *Application) sendAndWait(payload cast.Payload, sourceID, destinationID, namespace string) (*pb.CastMessage, error) {
	requestID, err := a.send(payload, sourceID, destinationID, namespace)
	if err != nil {
		return nil, err
	}

	// Set a timeout to wait for the response
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// TODO(vishen): not concurrent safe. Not a problem at the moment
	// because only synchronous flow currently allowed.
	resultChan := make(chan *pb.CastMessage, 1)
	a.resultChanMap[requestID] = resultChan
	defer func() {
		delete(a.resultChanMap, requestID)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChan:
		return result, nil
	}
}

// TODO(vishen): needing send(AndWait)* method seems a bit clunky, is there a better approach?
// Maybe having a struct that has send and sendAndWait, similar to before.
func (a *Application) sendDefaultConn(payload cast.Payload) error {
	_, err := a.send(payload, defaultSender, defaultRecv, namespaceConn)
	return err
}

func (a *Application) sendDefaultRecv(payload cast.Payload) error {
	_, err := a.send(payload, defaultSender, defaultRecv, namespaceRecv)
	return err
}

func (a *Application) sendMediaConn(payload cast.Payload) error {
	if a.application == nil {
		return ErrApplicationNotSet
	}
	_, err := a.send(payload, defaultSender, a.application.TransportId, namespaceConn)
	return err
}

func (a *Application) sendMediaRecv(payload cast.Payload) error {
	if a.application == nil {
		return ErrApplicationNotSet
	}
	_, err := a.send(payload, defaultSender, a.application.TransportId, namespaceMedia)
	return err
}

func (a *Application) sendAndWaitDefaultConn(payload cast.Payload) (*pb.CastMessage, error) {
	return a.sendAndWait(payload, defaultSender, defaultRecv, namespaceConn)
}

func (a *Application) sendAndWaitDefaultRecv(payload cast.Payload) (*pb.CastMessage, error) {
	return a.sendAndWait(payload, defaultSender, defaultRecv, namespaceRecv)
}

func (a *Application) sendAndWaitMediaConn(payload cast.Payload) (*pb.CastMessage, error) {
	if a.application == nil {
		return nil, ErrApplicationNotSet
	}
	return a.sendAndWait(payload, defaultSender, a.application.TransportId, namespaceConn)
}

func (a *Application) sendAndWaitMediaRecv(payload cast.Payload) (*pb.CastMessage, error) {
	if a.application == nil {
		return nil, ErrApplicationNotSet
	}
	return a.sendAndWait(payload, defaultSender, a.application.TransportId, namespaceMedia)
}

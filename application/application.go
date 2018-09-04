package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/buger/jsonparser"
	"github.com/pkg/errors"

	"github.com/vishen/go-chromecast/cast"
	pb "github.com/vishen/go-chromecast/cast/proto"
	castdns "github.com/vishen/go-chromecast/dns"
	"github.com/vishen/go-chromecast/storage"
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

type Application struct {
	conn *cast.Connection

	debugging bool

	// Current values from the chromecast
	application *cast.Application
	media       *cast.Media
	volume      *cast.Volume

	httpServer *http.Server
	serverPort int
	localIP    string

	// NOTE: Currently only playing one media file at a time is handled
	mediaFinished  chan bool
	mediaFilenames []string

	playedItems   map[string]PlayedItem
	cacheDisabled bool
	cache         *storage.Storage
}

func NewApplication(debugging, cacheDisabled bool) *Application {
	// TODO(vishen): make cast.Connection an interface, most likely will just need
	// the Send method
	return &Application{
		conn:          cast.NewConnection(debugging),
		debugging:     debugging,
		cacheDisabled: cacheDisabled,
		playedItems:   map[string]PlayedItem{},
		cache:         storage.NewStorage(),
	}
}

func (a *Application) Start(entry castdns.CastDNSEntry) error {
	if err := a.loadPlayedItems(); err != nil {
		a.debug("unable to load played items: %v", err)
	}

	if err := a.conn.Start(entry.GetAddr(), entry.GetPort()); err != nil {
		return errors.Wrap(err, "unable to start connection")
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
	if err != nil {
		return err
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
	recvStatus, err := a.getReceiverStatus()
	if err != nil {
		return err
	}

	/*
		TODO: this seems to happen semi-frequently, maybe add an exponetial retry?
		2018/08/26 10:47:56 [connection] sender-0 <- receiver-0 [urn:x-cast:com.google.cast.receiver]: {"requestId":2,"status":{"userEq":{"high_shelf":{"frequency":4500.0,"gain_db":0.0,"quality":0.707},"low_shelf":{"frequency":150.0,"gain_db":0.0,"quality":0.707},"max_peaking_eqs":0,"peaking_eqs":[]},"volume":{"controlType":"master","level":0.550000011920929,"muted":false,"stepInterval":0.019999999552965164}},"type":"RECEIVER_STATUS"}
		Error: unable to update application: no applications running
	*/

	if len(recvStatus.Status.Applications) > 1 {
		a.debug("more than 1 connected application on the chromecast: (%d)%#v", len(recvStatus.Status.Applications), recvStatus.Status.Applications)
	} else if len(recvStatus.Status.Applications) == 0 {
		return errors.New("no applications running")
	}
	// TODO(vishen): Why could there be more than one application, how to handle this?
	// For now just take the last one.
	for _, app := range recvStatus.Status.Applications {
		a.application = &app
	}
	a.volume = &recvStatus.Status.Volume

	if a.application.IsIdleScreen {
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
		a.volume = &media.Volume
	}
	return nil
}

func (a *Application) Close() {
	a.sendMediaConn(&cast.CloseHeader)
	a.sendDefaultConn(&cast.CloseHeader)
}

func (a *Application) Status() (*cast.Application, *cast.Media, *cast.Volume) {
	return a.application, a.media, a.volume
}

func (a *Application) Pause() error {
	if a.media == nil {
		return errors.New("media not yet initialised, there is nothing to pause")
	}
	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.PauseHeader,
		MediaSessionId: a.media.MediaSessionId,
	})
}

func (a *Application) Unpause() error {
	if a.media == nil {
		return errors.New("media not yet initialised, there is nothing to unpause")
	}
	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.PlayHeader,
		MediaSessionId: a.media.MediaSessionId,
	})
}

func (a *Application) StopMedia() error {
	if a.media == nil {
		return errors.New("media not yet initialised, there is nothing to stop")
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
		return errors.New("media not yet initialised, there is nothing to stop")
	}

	// TODO(vishen): Get the number of queue items, if none, possibly just skip to the end?
	_, err := a.sendAndWaitMediaRecv(&cast.QueueUpdate{
		PayloadHeader:  cast.QueueUpdateHeader,
		MediaSessionId: a.media.MediaSessionId,
		Jump:           1,
	})
	return err
}

func (a *Application) Skip() error {

	if a.media == nil {
		return errors.New("media not yet initialised, there is nothing to stop")
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
		return errors.New("media not yet initialised")
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
		return errors.New("media not yet initialised")
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

	switch ext := path.Ext(filename); ext {
	case ".mkv", ".mp4", ".m4a", ".m4p", ".MP4":
		return "video/mp4", nil
	case ".webm":
		return "video/webm", nil
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

func (a *Application) QueueLoad(filenames []string, contentType string, transcode bool) error {

	// TODO: Check all filenames
	filename := filenames[0]

	if _, err := os.Stat(filename); err != nil {
		return errors.Wrapf(err, "unable to find %q", filename)
	}
	/*
		We can play media for the following:

		- if we have a filename with a known content type
		- if we have a filename, and a specified contentType
		- if we have a filename with an unknown content type, and transcode is true
		-
	*/
	knownFileType := a.knownFileType(filename)
	if !knownFileType && contentType == "" && !transcode {
		return fmt.Errorf("unknown content-type for %q, either specify a content-type or set transcode to true", filename)
	}

	// Set the content-type
	if contentType != "" {
	} else if knownFileType {
		contentType, _ = a.possibleContentType(filename)
	} else if transcode {
		contentType = "video/mp4"
	}

	// TODO: maybe cache this somewhere
	localIP, err := a.getLocalIP()
	if err != nil {
		return err
	}

	a.debug("starting streaming server")

	// Start server to serve the media
	if err := a.startStreamingServer(); err != nil {
		return errors.Wrap(err, "unable to start streaming server")
	}

	a.debug("started streaming server")

	items := []cast.QueueLoadItem{}

	for _, f := range filenames {
		// Set the content url
		contentUrl := fmt.Sprintf("http://%s:%d?media_file=%s&live_streaming=%t", localIP, a.serverPort, f, transcode)
		a.mediaFilenames = append(a.mediaFilenames, f)
		items = append(items, cast.QueueLoadItem{
			Autoplay:         true,
			PlaybackDuration: 60,
			Media: cast.MediaItem{
				ContentId:   contentUrl,
				StreamType:  "BUFFERED",
				ContentType: contentType,
			},
		})
	}

	// If the current chromecast application isn't the Default Media Receiver
	// we need to change it
	if a.application.AppId != defaultChromecastAppId {
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

	// Send the command to the chromecast
	a.sendMediaRecv(&cast.QueueLoad{
		PayloadHeader: cast.QueueLoadHeader,
		CurrentTime:   0,
		StartIndex:    0,
		RepeatMode:    "REPEAT_OFF",
		Items:         items,
	})

	c := make(chan bool, 1)
	<-c
	return nil
}

func (a *Application) Load(filename, contentType string, transcode bool) error {

	if _, err := os.Stat(filename); err != nil {
		return errors.Wrapf(err, "unable to find %q", filename)
	}
	/*
		We can play media for the following:

		- if we have a filename with a known content type
		- if we have a filename, and a specified contentType
		- if we have a filename with an unknown content type, and transcode is true
		-
	*/
	knownFileType := a.knownFileType(filename)
	if !knownFileType && contentType == "" && !transcode {
		return fmt.Errorf("unknown content-type for %q, either specify a content-type or set transcode to true", filename)
	}

	// Set the content-type
	if contentType != "" {
	} else if knownFileType {
		contentType, _ = a.possibleContentType(filename)
	} else if transcode {
		contentType = "video/mp4"
		if err := exec.Command("ffmpeg", "-version").Run(); err != nil {
			return errors.Wrap(err, "unable to run ffmpeg, it is required to transcode videos")
		}
	}

	a.debug("starting streaming server")

	// Start server to serve the media
	if err := a.startStreamingServer(); err != nil {
		return errors.Wrap(err, "unable to start streaming server")
	}

	a.debug("started streaming server")

	// Get the local inet address so the chromecast can access it because assumably they
	// are on the same network
	localIP, err := a.getLocalIP()
	if err != nil {
		return err
	}

	// Set the content url
	contentUrl := fmt.Sprintf("http://%s:%d?media_file=%s&live_streaming=%t", localIP, a.serverPort, filename, transcode)
	url, _ := url.Parse(contentUrl)
	a.mediaFilenames = append(a.mediaFilenames, filename)

	// If the current chromecast application isn't the Default Media Receiver
	// we need to change it
	if a.application.AppId != defaultChromecastAppId {
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

	// Send the command to the chromecast
	a.sendMediaRecv(&cast.LoadMediaCommand{
		PayloadHeader: cast.LoadHeader,
		CurrentTime:   0,
		Autoplay:      true,
		Media: cast.MediaItem{
			ContentId:   url.String(),
			StreamType:  "BUFFERED",
			ContentType: contentType,
		},
	})

	// Wait until we have been notified that the media has finished playing
	<-a.mediaFinished
	return nil
}

func (a *Application) getLocalIP() (string, error) {
	if a.localIP != "" {
		return a.localIP, nil
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
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
	a.debug("trying to find available port to start streaming server on")

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return errors.Wrap(err, "unable to bind to local tcp address")
	}

	a.serverPort = listener.Addr().(*net.TCPAddr).Port
	a.debug("found available port :%d", a.serverPort)

	a.mediaFinished = make(chan bool, 1)
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

		a.debug("canServe=%t, liveStreaming=%t, filename=%s", canServe, liveStreaming, filename)
		if canServe {
			if !liveStreaming {
				http.ServeFile(w, r, filename)
			} else {
				a.serveLiveStreaming(w, r, filename)
			}
		} else {
			http.Error(w, "Invalid file", 400)
		}
		a.debug("method=%s, headers=%v, reponse_headers=%v", r.Method, r.Header, w.Header())
		pi := a.playedItems[filename]

		// TODO(vishen): make this a pointer?
		pi.Finished = time.Now().Unix()
		a.playedItems[filename] = pi
		a.writePlayedItems()
	})

	go func() {
		a.debug("media server listening on %d", a.serverPort)
		if err := a.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	return nil
}

func (a *Application) serveLiveStreaming(w http.ResponseWriter, r *http.Request, filename string) {
	cmd := exec.Command(
		"ffmpeg",
		"-i", filename,
		"-vcodec", "h264",
		"-f", "mp4",
		"-movflags", "frag_keyframe+faststart",
		"-strict", "-experimental",
		"pipe:1",
	)

	cmd.Stdout = w

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")

	if err := cmd.Run(); err != nil {
		log.Printf("error transcoding %q: %v\n", filename, err)
	}

}

func (a *Application) debug(message string, args ...interface{}) {
	if a.debugging {
		log.Printf("[application] %s", fmt.Sprintf(message, args...))
	}
}

func (a *Application) send(payload cast.Payload, sourceID, destinationID, namespace string) error {
	return a.conn.Send(payload, sourceID, destinationID, namespace)
}

func (a *Application) sendAndWait(payload cast.Payload, sourceID, destinationID, namespace string) (*pb.CastMessage, error) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	message, err := a.conn.SendAndWait(ctx, payload, sourceID, destinationID, namespace)
	if err != nil {
		return nil, err
	}

	// TODO(vishen): 	if the media application id changes, the media should act as finished
	// receiver-0 [urn:x-cast:com.google.cast.receiver]: {"requestId":505942120,"status":{"applications":[{"appId":"233637DE","displayName":"YouTube","isIdleScreen":false,"launchedFromCloud":false,"namespaces":[{"name":"urn:x-cast:com.google.cast.debugoverlay"},{"name":"urn:x-cast:com.google.cast.cac"},{"name":"urn:x-cast:com.google.cast.media"},{"name":"urn:x-cast:com.google.youtube.mdx"}],"sessionId":"89efac28-6d0c-420f-9c78-39175dbcae84","statusText":"YouTube","transportId":"89efac28-6d0c-420f-9c78-39175dbcae84"}],"volume":{"controlType":"attenuation","level":1.0,"muted":false,"stepInterval":0.05000000074505806}},"type":"RECEIVER_STATUS"}

	messageBytes := []byte(*message.PayloadUtf8)
	messageType, err := jsonparser.GetString(messageBytes, "type")
	if err != nil {
		// We don't really care if there is an error here, just let
		// the caller handle the message
		return message, nil
	}
	// Happens when the chromecast was unable to process the file served
	if messageType == "LOAD_FAILED" {
		a.mediaFinished <- true

	} else if messageType == "MEDIA_STATUS" {
		mediaStatusResponse := cast.MediaStatusResponse{}
		if err := json.Unmarshal(messageBytes, &mediaStatusResponse); err == nil {
			for _, status := range mediaStatusResponse.Status {
				if status.IdleReason == "FINISHED" {
					a.mediaFinished <- true
					return message, nil
				}
			}
		}
	}
	return message, nil
}

// TODO(vishen): needing send(AndWait)* method seems a bit clunky, is there a better approach?
// Maybe having a struct that has send and sendAndWait, similar to before.
func (a *Application) sendDefaultConn(payload cast.Payload) error {
	return a.send(payload, defaultSender, defaultRecv, namespaceConn)
}

func (a *Application) sendDefaultRecv(payload cast.Payload) error {
	return a.send(payload, defaultSender, defaultRecv, namespaceRecv)
}

func (a *Application) sendMediaConn(payload cast.Payload) error {
	if a.application == nil {
		return errors.New("application isn't set")
	}
	return a.send(payload, defaultSender, a.application.TransportId, namespaceConn)
}

func (a *Application) sendMediaRecv(payload cast.Payload) error {
	if a.application == nil {
		return errors.New("application isn't set")
	}
	return a.send(payload, defaultSender, a.application.TransportId, namespaceMedia)
}

func (a *Application) sendAndWaitDefaultConn(payload cast.Payload) (*pb.CastMessage, error) {
	return a.sendAndWait(payload, defaultSender, defaultRecv, namespaceConn)
}

func (a *Application) sendAndWaitDefaultRecv(payload cast.Payload) (*pb.CastMessage, error) {
	return a.sendAndWait(payload, defaultSender, defaultRecv, namespaceRecv)
}

func (a *Application) sendAndWaitMediaConn(payload cast.Payload) (*pb.CastMessage, error) {
	if a.application == nil {
		return nil, errors.New("application isn't set")
	}
	return a.sendAndWait(payload, defaultSender, a.application.TransportId, namespaceConn)
}

func (a *Application) sendAndWaitMediaRecv(payload cast.Payload) (*pb.CastMessage, error) {
	if a.application == nil {
		return nil, errors.New("application isn't set")
	}
	return a.sendAndWait(payload, defaultSender, a.application.TransportId, namespaceMedia)
}

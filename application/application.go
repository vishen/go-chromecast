package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/buger/jsonparser"
	"github.com/pkg/errors"

	"github.com/vishen/go-chromecast/cast"
	pb "github.com/vishen/go-chromecast/cast/proto"
	castdns "github.com/vishen/go-chromecast/dns"
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

type Application struct {
	conn *cast.Connection

	debugging bool

	// Current values from the chromecast
	application *cast.Application
	media       *cast.Media
	volume      *cast.Volume

	httpServer *http.Server

	// NOTE: Currently only playing one media file at a time is handled
	playMediaFinished  chan bool
	playMediaFilenames []string
}

func NewApplication(debugging bool) *Application {
	// TODO(vishen): make cast.Connection an interface, most likely will just need
	// the Send method
	return &Application{
		conn:      cast.NewConnection(debugging),
		debugging: debugging,
	}
}

func (a *Application) Start(entry castdns.CastDNSEntry) error {
	if err := a.conn.Start(entry.GetAddr(), entry.GetPort()); err != nil {
		return errors.Wrap(err, "unable to start connection")
	}
	if err := a.sendDefaultConn(&cast.ConnectHeader); err != nil {
		return errors.Wrap(err, "unable to connect to chromecast")
	}
	return errors.Wrap(a.Update(), "unable to update application")
}

func (a *Application) Update() error {
	recvStatus, err := a.getReceiverStatus()
	if err != nil {
		return err
	}

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

func (a *Application) Close() {
	a.sendMediaConn(&cast.CloseHeader)
	a.sendDefaultConn(&cast.CloseHeader)
}

func (a *Application) Pause() error {
	if a.media == nil {
		return errors.New("media not yet initialised")
	}
	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.PauseHeader,
		MediaSessionId: a.media.MediaSessionId,
	})
}

func (a *Application) Unpause() error {
	if a.media == nil {
		return errors.New("media not yet initialised")
	}
	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.PlayHeader,
		MediaSessionId: a.media.MediaSessionId,
	})
}

func (a *Application) Seek(value int) error {
	if a.media == nil {
		return errors.New("media not yet initialised")
	}

	// Get the latest media status
	// TODO(vishen): can we unroll this, so it doesn't update the current state?
	// but just returns it?
	// that might also make a.media == nil checks pointless?
	a.updateMediaStatus()

	var currentTime float32 = 0.0
	if value != 0 {
		currentTime = a.media.CurrentTime + float32(value)
		if a.media.Media.Duration < currentTime {
			currentTime = a.media.Media.Duration - 2
		} else if currentTime < 0 {
			currentTime = 0
		}
	}

	// TODO(vishen): maybe there is another ResumeState that lets us
	// seek from the end? Although not sure how this works for live media?

	return a.sendMediaRecv(&cast.MediaHeader{
		PayloadHeader:  cast.SeekHeader,
		MediaSessionId: a.media.MediaSessionId,
		CurrentTime:    currentTime,
		ResumeState:    "PLAYBACK_START",
	})
}

func (a *Application) debug(message string, args ...interface{}) {
	if a.debugging {
		log.Printf("[application] %s", fmt.Sprintf(message, args))
	}
}

func (a *Application) send(payload cast.Payload, sourceID, destinationID, namespace string) error {
	return a.conn.Send(payload, sourceID, destinationID, namespace)
}

func (a *Application) sendAndWait(payload cast.Payload, sourceID, destinationID, namespace string) (*pb.CastMessage, error) {

	// TODO(vishen): make context a timeout with some sensible default, and be configurable
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	// TODO(vishen): Make another send function on cast/connection that won't wait
	// for a return message, as the initial CONNECT doesn't seem to respond...
	message, err := a.conn.SendAndWait(ctx, payload, sourceID, destinationID, namespace)
	if err != nil {
		return nil, err
	}

	messageBytes := []byte(*message.PayloadUtf8)
	messageType, err := jsonparser.GetString(messageBytes, "type")
	if err != nil {
		// We don't really care if there is an error here, just let
		// the caller handle the message
		return message, nil
	}
	// Happens when the chromecast was unable to process the file served
	if messageType == "LOAD_FAILED" {
		//ca.playMediaFinished <- true

	} else if messageType == "MEDIA_STATUS" {
		/*mediaStatusResponse := MediaStatusResponse{}
		if err := json.Unmarshal(messageBytes, &mediaStatusResponse); err == nil {
			for _, status := range mediaStatusResponse.Status {
				if status.IdleReason == "FINISHED" {
					ca.playMediaFinished <- true
					return true
				}
			}
		}*/
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

/*

func (ca *CastApplication) startServer() {
	if ca.httpServer != nil {
		return
	}

	ca.playMediaFinished = make(chan bool, 1)
	ca.httpServer = &http.Server{Addr: "0.0.0.0" + port}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check to see if we have a 'filename' and if it is one of the ones that have
		// already been validated and is useable.
		filename := r.URL.Query().Get("media_file")
		canServe := false
		for _, fn := range ca.playMediaFilenames {
			if fn == filename {
				canServe = true
			}
		}

		// Check to see if this is a live streaming video and we need to use an
		// infinite range request / response. This comes from media that is either
		// live or currently being transcoded to a different media format.
		liveStreaming := false
		if ls := r.URL.Query().Get("live_streaming"); ls == "true" {
			liveStreaming = true
		}

		fmt.Printf("canServe=%t, liveStreaming=%t, filename=%s\n", canServe, liveStreaming, filename)
		if canServe {
			if !liveStreaming {
				http.ServeFile(w, r, filename)
			} else {
				ca.serveLiveStreaming(w, r, filename)
			}
		} else {
			http.Error(w, "Invalid file", 400)
		}
		fmt.Printf("method=%s, headers=%v, reponse_headers=%v\n", r.Method, r.Header, w.Header())

	})

	go func() {
		fmt.Printf("Media server listening on %s\n", port)
		if err := ca.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

		// TODO(vishen): 	if the media application id changes, the media should act as finished

		// receiver-0 [urn:x-cast:com.google.cast.receiver]: {"requestId":505942120,"status":{"applications":[{"appId":"233637DE","displayName":"YouTube","isIdleScreen":false,"launchedFromCloud":false,"namespaces":[{"name":"urn:x-cast:com.google.cast.debugoverlay"},{"name":"urn:x-cast:com.google.cast.cac"},{"name":"urn:x-cast:com.google.cast.media"},{"name":"urn:x-cast:com.google.youtube.mdx"}],"sessionId":"89efac28-6d0c-420f-9c78-39175dbcae84","statusText":"YouTube","transportId":"89efac28-6d0c-420f-9c78-39175dbcae84"}],"volume":{"controlType":"attenuation","level":1.0,"muted":false,"stepInterval":0.05000000074505806}},"type":"RECEIVER_STATUS"}

	// Add a message handler to listen for any messages received that would indicate that
	// the media has finished
	ca.castConn.addMessageHandler(func(message *api.CastMessage) bool {
		messageBytes := []byte(*message.PayloadUtf8)
		messageType, err := jsonparser.GetString(messageBytes, "type")
		if err != nil {
			return false
		}
		// Happens when the chromecast was unable to process the file served
		if messageType == "LOAD_FAILED" {
			ca.playMediaFinished <- true
			return true
		} else if messageType == "MEDIA_STATUS" {
			mediaStatusResponse := MediaStatusResponse{}
			if err := json.Unmarshal(messageBytes, &mediaStatusResponse); err == nil {
				for _, status := range mediaStatusResponse.Status {
					if status.IdleReason == "FINISHED" {
						ca.playMediaFinished <- true
						return true
					}
				}
			}
		}
		return false
	})

}

func (ca *CastApplication) serveLiveStreaming(w http.ResponseWriter, r *http.Request, filename string) {
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
func (ca *CastApplication) closeServer() {
	if ca.httpServer == nil {
		return
	}

	ca.httpServer.Shutdown(nil)
}

func (ca *CastApplication) PlayMedia(filenameOrUrl, contentType string, liveStreaming bool) error {

	// Check that we have a valid content type as the chromecast default media reciever
	// only handles a limited number of content types.
	if !ca.CanUseContentType(contentType) {
		return fmt.Errorf("Unknown content type '%s'", contentType)
	}

	// The url for the chromecast to stream off
	var contentUrl string

	// If we have a url just use that, if we have a filename we need to
	// start a local server to stream the file and use that url to send
	// to the chromecast for it to stream off
	if _, err := os.Stat(filenameOrUrl); err == nil {

		// Start server to serve the media
		ca.startServer()

		// Get the local inet address so the chromecast can access it because assumably they
		// are on the same network
		localIP, err := getLocalIP()
		if err != nil {
			return err
		}

		// Set the content url
		contentUrl = fmt.Sprintf("http://%s%s?media_file=%s&live_streaming=%t", localIP, port, filenameOrUrl, liveStreaming)
		ca.playMediaFilenames = append(ca.playMediaFilenames, filenameOrUrl)

	} else if _, err := url.ParseRequestURI(filenameOrUrl); err == nil {
		contentUrl = filenameOrUrl
	} else {
		return fmt.Errorf("'%s' is not a valid file or url", filenameOrUrl)
	}

	// If the current chromecast application isn't the Default Media Receiver
	// we need to change it
	if ca.application.AppId != defaultChromecastAppId {
		_, err := ca.defaultRecv.SendAndWait(&LaunchRequest{
			PayloadHeader: launchHeader,
			AppId:         defaultChromecastAppId,
		})

		if err != nil {
			return err
		}

		// Update the 'application' and 'media' field on the 'CastApplication'
		ca.Update()
	}

	// Send the command to the chromecast
	ca.mediaRecv.Send(&LoadMediaCommand{
		PayloadHeader: loadHeader,
		CurrentTime:   0,
		Autoplay:      true,
		Media: MediaItem{
			ContentId:   contentUrl,
			StreamType:  "BUFFERED",
			ContentType: contentType,
		},
	})

	// Wait until we have been notified that the media has finished playing
	<-ca.playMediaFinished

	fmt.Println("Finished media")

	return nil

}
func getLikelyContentType(filename string) (string, error) {
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
		return "", fmt.Errorf("Unknown file extension '%s'", ext)
	}
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("Failed to get local ip address")
}

*/

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/buger/jsonparser"
	"github.com/vishen/go-chromecast/api"
)

const (
	// 'CC1AD845' seems to be a predefined app; check link
	// https://gist.github.com/jloutsenhizer/8855258
	// https://github.com/thibauts/node-castv2
	defaultChromecastAppId = "CC1AD845"

	// TODO(vishen): Randomise or let it be set at runtime
	port = ":34455"
)

var (
	defaultSender = "sender-0"
	defaultRecv   = "receiver-0"

	namespaceConn  = "urn:x-cast:com.google.cast.tp.connection"
	namespaceRecv  = "urn:x-cast:com.google.cast.receiver"
	namespaceMedia = "urn:x-cast:com.google.cast.media"

	knownContentTypes = []string{
		"video/webm",
		"video/mp4",
	}
)

type CastApplication struct {
	castConn    *CastConnection
	defaultConn *CastInterface
	defaultRecv *CastInterface
	mediaConn   *CastInterface
	mediaRecv   *CastInterface

	application Application
	media       Media
	volume      Volume

	httpServer *http.Server

	// NOTE: Currently only playing one media file at a time is handled
	playMediaFinished  chan bool
	playMediaFilenames []string
}

func NewCastApplication(castConn *CastConnection) *CastApplication {
	return &CastApplication{
		castConn:    castConn,
		defaultConn: NewCastInterface(castConn, defaultSender, defaultRecv, namespaceConn),
		defaultRecv: NewCastInterface(castConn, defaultSender, defaultRecv, namespaceRecv),
	}
}

func (ca *CastApplication) Start() error {
	if err := ca.defaultConn.Send(&connectHeader); err != nil {
		return err
	}

	return ca.Update()
}

func (ca *CastApplication) Update() error {
	recvStatus, err := ca.getReceiverStatus()
	if err != nil {
		return err
	}

	// TODO(vishen): Why could there be more than one application, how to handle this?
	// For now just take the last one.
	for _, app := range recvStatus.Status.Applications {
		ca.application = app
	}
	ca.volume = recvStatus.Status.Volume

	if ca.application.IsIdleScreen {
		return nil
	}

	ca.mediaConn = NewCastInterface(ca.castConn, defaultSender, ca.application.TransportId, namespaceConn)
	ca.mediaRecv = NewCastInterface(ca.castConn, defaultSender, ca.application.TransportId, namespaceMedia)

	ca.updateMediaStatus()

	return nil

}

func (ca *CastApplication) updateMediaStatus() error {

	ca.mediaConn.Send(&connectHeader)

	mediaStatus, err := ca.getMediaStatus()
	if err != nil {
		return err
	}

	for _, media := range mediaStatus.Status {
		ca.media = media
		ca.volume = media.Volume
	}

	return nil
}

func (ca *CastApplication) getMediaStatus() (*MediaStatusResponse, error) {

	apiMessage, err := ca.mediaRecv.SendAndWait(&getStatusHeader)
	if err != nil {
		return nil, err
	}

	var response MediaStatusResponse
	if err := json.Unmarshal([]byte(*apiMessage.PayloadUtf8), &response); err != nil {
		return nil, fmt.Errorf("error unmarshaling json: %s", err)
	}

	return &response, nil
}

func (ca *CastApplication) getReceiverStatus() (*ReceiverStatusResponse, error) {
	apiMessage, err := ca.defaultRecv.SendAndWait(&getStatusHeader)
	if err != nil {
		return nil, err
	}

	var response ReceiverStatusResponse
	if err := json.Unmarshal([]byte(*apiMessage.PayloadUtf8), &response); err != nil {
		return nil, fmt.Errorf("error unmarshaling json: %s", err)
	}

	return &response, nil

}

func (ca *CastApplication) Close() {
	ca.closeServer()
	if ca.mediaConn != nil {
		ca.mediaConn.Send(&closeHeader)
	}
	ca.defaultConn.Send(&closeHeader)
}

func (ca *CastApplication) Pause() {

	if ca.mediaConn == nil {
		return
	}

	ca.mediaRecv.Send(&MediaHeader{
		PayloadHeader:  pauseHeader,
		MediaSessionId: ca.media.MediaSessionId,
	})
}

func (ca *CastApplication) Unpause() {
	if ca.mediaConn == nil {
		return
	}

	ca.mediaRecv.Send(&MediaHeader{
		PayloadHeader:  playHeader,
		MediaSessionId: ca.media.MediaSessionId,
	})
}

func (ca *CastApplication) Seek(value int) {

	if ca.mediaConn == nil {
		return
	}

	ca.updateMediaStatus()

	var currentTime float32 = 0.0
	if value != 0 {
		currentTime = ca.media.CurrentTime + float32(value)
		if ca.media.Media.Duration < currentTime {
			currentTime = ca.media.Media.Duration - 2
		} else if currentTime < 0 {
			currentTime = 0
		}
	}

	ca.mediaRecv.Send(&MediaHeader{
		PayloadHeader:  seekHeader,
		MediaSessionId: ca.media.MediaSessionId,
		CurrentTime:    currentTime,
		ResumeState:    "PLAYBACK_START",
	})
}

func (ca *CastApplication) CanUseContentType(contentType string) bool {
	for _, kct := range knownContentTypes {
		if kct == contentType {
			return true
		}
	}
	return false
}

func (ca *CastApplication) startServer() {
	if ca.httpServer != nil {
		return
	}

	ca.playMediaFinished = make(chan bool, 1)
	ca.httpServer = &http.Server{Addr: "0.0.0.0" + port}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Handling http request\n")
		filename := r.URL.Query().Get("media_file")

		canServe := false
		for _, fn := range ca.playMediaFilenames {
			if fn == filename {
				canServe = true
			}
		}

		if canServe {
			http.ServeFile(w, r, filename)
		} else {
			http.Error(w, "Invalid file", 400)
		}

	})

	go func() {
		fmt.Printf("Media server listening on %s\n", port)
		if err := ca.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

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

func (ca *CastApplication) closeServer() {
	if ca.httpServer == nil {
		return
	}

	ca.httpServer.Shutdown(nil)
}

func (ca *CastApplication) PlayMedia(filenameOrUrl, contentType string) error {

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
		contentUrl = fmt.Sprintf("http://%s%s?media_file=%s", localIP, port, filenameOrUrl)
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
		CurrentTime:   1240,
		Autoplay:      true,
		Media: MediaItem{
			ContentId:   contentUrl,
			StreamType:  "BUFFERED",
			ContentType: contentType,
		},
	})

	// Wait until we have been notified that the media has finished playing
	<-ca.playMediaFinished

	return nil

}

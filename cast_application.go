package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/vishen/go-chromecast/api"
)

const (
	// 'CC1AD845' seems to be a predefined app; check link
	// https://gist.github.com/jloutsenhizer/8855258
	// https://github.com/thibauts/node-castv2
	defaultChromecastAppId = "CC1AD845"
)

var (
	defaultSender = "sender-0"
	defaultRecv   = "receiver-0"

	namespaceConn  = "urn:x-cast:com.google.cast.tp.connection"
	namespaceRecv  = "urn:x-cast:com.google.cast.receiver"
	namespaceMedia = "urn:x-cast:com.google.cast.media"
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
		}
	}

	ca.mediaRecv.Send(&MediaHeader{
		PayloadHeader:  seekHeader,
		MediaSessionId: ca.media.MediaSessionId,
		CurrentTime:    currentTime,
		ResumeState:    "PLAYBACK_START",
	})
}

func (ca *CastApplication) PlayMedia(filename string) error {

	fileParts := strings.Split(filename, ".")
	if fileParts[len(fileParts)-1] != "mp4" {
		return fmt.Errorf("File is not an .mp4 extenstion")
	}

	localIP, err := getLocalIP()
	if err != nil {
		return err
	}

	if ca.application.AppId != defaultChromecastAppId {
		_, err := ca.defaultRecv.SendAndWait(&LaunchRequest{
			PayloadHeader: launchHeader,
			AppId:         defaultChromecastAppId,
		})

		if err != nil {
			return err
		}

		ca.Update()
	}

	done := make(chan bool, 1)
	port := ":34455"

	// TODO(vishen): Randomise port?
	srv := &http.Server{Addr: "0.0.0.0" + port}

	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("Handling http request\n")
			http.ServeFile(w, r, filename)
		})
		fmt.Printf("Media server listening on %s\n", port)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	// TODO(vishen): There must be a better way than just adding message handlers everywhere?
	// 				 If not at least clean up the api so it is easier to understand.
	// TODO(vishen): This needs to be cleaned up somehow? dont want to have these
	// 				 message handlers just hanging around
	mh := func(message *api.CastMessage) bool {
		messageBytes := []byte(*message.PayloadUtf8)
		messageType, err := jsonparser.GetString(messageBytes, "type")
		if err != nil || messageType != "MEDIA_STATUS" {
			return false
		}

		mediaStatusResponse := MediaStatusResponse{}
		if err := json.Unmarshal(messageBytes, &mediaStatusResponse); err == nil {
			for _, status := range mediaStatusResponse.Status {
				if status.IdleReason == "FINISHED" {
					done <- true
					return true
				}
			}
		}

		return false

	}
	ca.castConn.addMessageHandler(mh)

	contentType := "video/mp4"
	ca.mediaRecv.Send(&LoadMediaCommand{
		PayloadHeader: loadHeader,
		CurrentTime:   0,
		Autoplay:      true,
		Media: MediaItem{
			ContentId:   "http://" + localIP + port,
			StreamType:  "BUFFERED",
			ContentType: contentType,
		},
	})

	<-done
	srv.Shutdown(nil)

	return nil

}

package main

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/barnybug/go-cast/api"
	"github.com/buger/jsonparser"
	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/mdns"
)

var (
	requestId int
)

type Payload interface {
	SetRequestId(id int)
}

type PayloadHeader struct {
	Type      string `json:"type"`
	RequestId int    `json:"requestId,omitempty"`
}

func (p *PayloadHeader) SetRequestId(id int) {
	p.RequestId = id
}

type MediaHeader struct {
	PayloadHeader
	MediaSessionId int `json:"mediaSessionId"`
}

type Volume struct {
	Level float32 `json:"level"`
	Muted bool    `json:"muted"`
}

type ReceiverStatusResponse struct {
	PayloadHeader
	Status struct {
		Applications []struct {
			AppId        string `json:"appId"`
			DisplayName  string `json:"displayName"`
			IsIdleScreen bool   `json:"isIdleScreen"`
			SessionId    string `json:"sessionId"`
			StatusText   string `json:"statusText"`
			TransportId  string `json:"transportId"`
		} `json:"applications"`

		Volume Volume `json:"volume"`
	} `json:"status"`
}

type Application struct {
	AppId        string `json:"appId"`
	DisplayName  string `json:"displayName"`
	IsIdleScreen bool   `json:"isIdleScreen"`
	SessionId    string `json:"sessionId"`
	StatusText   string `json:"statusText"`
	TransportId  string `json:"transportId"`
}

type ReceiverStatusRequest struct {
	PayloadHeader
	Applications []Application `json:"applications"`

	Volume Volume `json:"volume"`
}

type LaunchRequest struct {
	PayloadHeader
	AppId string `json:"appId"`
}

type LoadMediaCommand struct {
	PayloadHeader
	Media       MediaItem   `json:"media"`
	CurrentTime int         `json:"currentTime"`
	Autoplay    bool        `json:"autoplay"`
	CustomData  interface{} `json:"customData"`
}

type MediaItem struct {
	ContentId   string  `json:"contentId"`
	ContentType string  `json:"contentType"`
	StreamType  string  `json:"streamType"`
	Duration    float32 `json:"duration"`
	Metadata    struct {
		MetadataType int    `json:"metadataType`
		Title        string `json:"title"`
		SongName     string `json:"songName"`
		Artist       string `json:"artist"`
	} `json:"metadata"`
}

type Media struct {
	MediaSessionId int     `json:"mediaSessionId"`
	PlayerState    string  `json:"playerState"`
	CurrentTime    float32 `json:"currentTime"`
	Volume         Volume  `json:"volume"`

	Media MediaItem `json:"media"`
}

type MediaStatusResponse struct {
	PayloadHeader
	Status []Media `json:"status"`
}

type CastConnection struct {
	conn   *tls.Conn
	addrV4 net.IP
	addrV6 net.IP
	port   int

	name string
	host string

	uuid       string
	device     string
	status     string
	deviceName string
	infoFields map[string]string

	appChan   chan Application
	mediaChan chan Media
}

func NewCastConnection(entry *mdns.ServiceEntry) *CastConnection {

	infoFields := make(map[string]string, len(entry.InfoFields))
	for _, infoField := range entry.InfoFields {
		splitField := strings.Split(infoField, "=")
		if len(splitField) != 2 {
			fmt.Printf("Incorrect format for field in entry.InfoFields: %s\n", infoField)
			continue
		}
		infoFields[splitField[0]] = splitField[1]
	}

	return &CastConnection{
		addrV4:     entry.AddrV4,
		addrV6:     entry.AddrV6,
		port:       entry.Port,
		name:       entry.Name,
		host:       entry.Host,
		infoFields: infoFields,
		uuid:       infoFields["id"],
		device:     infoFields["md"],
		deviceName: infoFields["fn"],
		status:     infoFields["rs"],
		appChan:    make(chan Application, 1),
		mediaChan:  make(chan Media, 1),
	}
}

func (cc *CastConnection) connect() error {
	var err error
	dialer := &net.Dialer{
		Timeout:   time.Second * 30,
		KeepAlive: time.Second * 30,
	}
	cc.conn, err = tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", cc.addrV4, cc.port), &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Errorf("Failed to connect to Chromecast: %s", err)
	}

	return nil
}

func (cc *CastConnection) send(payload interface{}, sourceId, destinationId, namespace string) error {

	payloadJson, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	payloadString := string(payloadJson)
	message := &api.CastMessage{
		ProtocolVersion: api.CastMessage_CASTV2_1_0.Enum(),
		SourceId:        &sourceId,
		DestinationId:   &destinationId,
		Namespace:       &namespace,
		PayloadType:     api.CastMessage_STRING.Enum(),
		PayloadUtf8:     &payloadString,
	}

	proto.SetDefaults(message)

	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	log.Printf("%s ⇒ %s [%s]: %s", *message.SourceId, *message.DestinationId, *message.Namespace, *message.PayloadUtf8)

	err = binary.Write(cc.conn, binary.BigEndian, uint32(len(data)))
	if err != nil {
		return err
	}
	_, err = cc.conn.Write(data)
	return err
}

func (cc *CastConnection) receiveLoop() {
	for {
		fmt.Printf("Waiting for message from chromecast...\n")
		var length uint32
		err := binary.Read(cc.conn, binary.BigEndian, &length)
		if err != nil {
			log.Printf("Failed to read packet length: %s", err)
			break
		}
		if length == 0 {
			log.Println("Empty packet received")
			continue
		}

		packet := make([]byte, length)
		i, err := io.ReadFull(cc.conn, packet)
		if err != nil {
			log.Printf("Failed to read packet: %s", err)
			break
		}

		if i != int(length) {
			log.Printf("Invalid packet size. Wanted: %d Read: %d", length, i)
			break
		}

		message := &api.CastMessage{}
		err = proto.Unmarshal(packet, message)
		if err != nil {
			log.Printf("Failed to unmarshal CastMessage: %s", err)
			break
		}

		log.Printf("%s ⇐ %s [%s]: %+v", *message.DestinationId, *message.SourceId, *message.Namespace, *message.PayloadUtf8)

		var headers PayloadHeader
		err = json.Unmarshal([]byte(*message.PayloadUtf8), &headers)

		if err != nil {
			log.Printf("Failed to unmarshal message: %s", err)
			break
		}

		cc.handleMessage(message, &headers)
	}
}

func (cc *CastConnection) handleMessage(message *api.CastMessage, headers *PayloadHeader) {

	messageType, err := jsonparser.GetString([]byte(*message.PayloadUtf8), "type")
	if err != nil {
		fmt.Printf("Could not find 'type' key in response: %s\n", err)
		return
	}

	switch messageType {
	case "PING":
		var pong = PayloadHeader{Type: "PONG"}
		if err := cc.send(pong, *message.SourceId, *message.DestinationId, *message.Namespace); err != nil {
			fmt.Printf("Error sending message: %s\n", err)
		}
	case "RECEIVER_STATUS":
		var response ReceiverStatusResponse
		if err := json.Unmarshal([]byte(*message.PayloadUtf8), &response); err != nil {
			fmt.Printf("Message=%+v\n", message)
			fmt.Printf("Headers=%+v\n", headers)
			fmt.Printf("error unmarshaling json: %s\n", err)
			return
		}
		for _, app := range response.Status.Applications {
			cc.appChan <- app
		}
	case "MEDIA_STATUS":
		var response MediaStatusResponse
		if err := json.Unmarshal([]byte(*message.PayloadUtf8), &response); err != nil {
			fmt.Printf("Message=%+v\n", message)
			fmt.Printf("Headers=%+v\n", headers)
			fmt.Printf("error unmarshaling json: %s\n", err)
			return
		}

		for _, media := range response.Status {
			cc.mediaChan <- media
		}
	default:
		fmt.Printf("Unknown response type '%s'\n", messageType)
		fmt.Printf("Message=%+v\n", message)
		fmt.Printf("Headers=%+v\n", headers)
	}
}

type CastInterface struct {
	castConnection *CastConnection

	sourceId      string
	destinationId string
	namespace     string
}

func NewCastInterface(castConnection *CastConnection, sourceId, destinationId, namespace string) *CastInterface {
	return &CastInterface{
		castConnection: castConnection,
		sourceId:       sourceId,
		destinationId:  destinationId,
		namespace:      namespace,
	}
}

func (cc *CastInterface) send(payload Payload) error {

	// NOTE: Not concurrent safe, but currently only synchronous flow is possible
	requestId += 1
	payload.SetRequestId(requestId)

	return cc.castConnection.send(payload, cc.sourceId, cc.destinationId, cc.namespace)
}

func videoHandler() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Handling http request\n")
		http.ServeFile(w, r, "/Users/jonathanpentecost/Downloads/office_season_2/the_dundies.mp4")
	})
	fmt.Printf("Media server listening on :8082\n")
	http.ListenAndServe("0.0.0.0:8082", nil)
}

func main() {
	go videoHandler()
	entriesCh := make(chan *mdns.ServiceEntry, 1)

	mdns.Query(&mdns.QueryParam{
		Service: "_googlecast._tcp",
		Domain:  "local",
		Timeout: time.Second * 3,
		Entries: entriesCh,
	})

	entry := <-entriesCh

	cc := NewCastConnection(entry)
	cc.connect()
	go cc.receiveLoop()

	defaultConn := NewCastInterface(cc, "sender-0", "receiver-0", "urn:x-cast:com.google.cast.tp.connection")
	defaultRecv := NewCastInterface(cc, "sender-0", "receiver-0", "urn:x-cast:com.google.cast.receiver")

	defaultConn.send(&PayloadHeader{Type: "CONNECT"})
	defaultRecv.send(&PayloadHeader{Type: "GET_STATUS"})
	app := <-cc.appChan
	fmt.Printf("App=%+v\n", app)

	if app.IsIdleScreen {
		fmt.Printf("Chromecast is in idle mode\n")
		return
	}

	streamVideo := false
	appId := "CC1AD845"
	//appId = "44F5A7A4"

	if streamVideo && app.AppId != appId {
		// AppId=CC1AD845 seems to be a predefined app; check link
		// https://gist.github.com/jloutsenhizer/8855258
		// https://github.com/thibauts/node-castv2
		defaultRecv.send(&LaunchRequest{PayloadHeader: PayloadHeader{Type: "LAUNCH"}, AppId: appId})
		app = <-cc.appChan
		fmt.Printf("App(2)=%+v\n", app)

	}

	// This is the current running application connections
	appConn := NewCastInterface(cc, "sender-0", app.TransportId, "urn:x-cast:com.google.cast.tp.connection")
	appMedia := NewCastInterface(cc, "sender-0", app.TransportId, "urn:x-cast:com.google.cast.media")

	appConn.send(&PayloadHeader{Type: "CONNECT"})

	// TODO(vishen): There is no response to "GET_STATUS" if there is nothing playing.
	appMedia.send(&PayloadHeader{Type: "GET_STATUS"})
	media := <-cc.mediaChan
	fmt.Printf("Media=%+v\n", media)

	/*
		url := args[0]
		contentType := "audio/mpeg"
		if len(args) > 1 {
			contentType = args[1]
		}
		item := controllers.MediaItem{
			ContentId:   url,
			StreamType:  "BUFFERED",
			ContentType: contentType,
		}
		_, err = media.LoadMedia(ctx, item, 0, true, map[string]interface{}{})
	*/

	/*
		Supported Media formats:
		AAC
		MP3
		MP4
		WAV
		WebM
	*/

	/*appMedia.send(&LoadMediaCommand{
		PayloadHeader: PayloadHeader{Type: "LOAD"},
		CurrentTime:   0,
		Autoplay:      true,
		Media: MediaItem{
			ContentId:   "http://192.168.0.31:8082",
			StreamType:  "BUFFERED",
			ContentType: "video/mp4", // Required annoyingly...
			//ContentType: "video/x-msvideo",
		},
	})*/

	//appMedia.send(&MediaHeader{PayloadHeader: PayloadHeader{Type: "PLAY"}, MediaSessionId: media.MediaSessionId})
	//appMedia.send(&MediaHeader{PayloadHeader: PayloadHeader{Type: "PAUSE"}, MediaSessionId: media.MediaSessionId})

	/*time.Sleep(time.Second * 15)
	appConn.send(&PayloadHeader{Type: "CLOSE"})

	defaultConn.send(&PayloadHeader{Type: "CLOSE"})*/

	done := make(chan bool)

	<-done

}

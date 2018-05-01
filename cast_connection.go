package main

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/buger/jsonparser"
	"github.com/gogo/protobuf/proto"
	"github.com/vishen/go-chromecast/api"
)

type MessageHandler func(*api.CastMessage) bool

type CastConnection struct {
	*CastEntry
	conn *tls.Conn

	mhLock          sync.Mutex
	messageHandlers []MessageHandler

	debug bool
}

func NewCastConnection(debug bool) *CastConnection {

	log.Println("Getting dns entry")
	// Find the dns entry for the chromecast
	entry := getCastEntry()
	log.Println("Got dns entry")

	cc := &CastConnection{
		CastEntry:       entry,
		mhLock:          sync.Mutex{},
		messageHandlers: make([]MessageHandler, 0),
		debug:           debug,
	}
	cc.log("debug", "connection info: %s", cc.String())
	return cc
}

func (cc *CastConnection) addMessageHandler(f MessageHandler) {
	cc.mhLock.Lock()
	cc.messageHandlers = append(cc.messageHandlers, f)
	cc.mhLock.Unlock()
}

func (cc *CastConnection) log(level, message string, args ...interface{}) {
	if cc.debug || level == "error" {
		// TODO(vishen): I was sure I could just pass everything straight
		// to 'log.Printf' like below, but it is failing?
		// log.Printf("[%s] "+message, level, args...)
		log.Printf(fmt.Sprintf("[%s] %s", level, message), args...)
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

	cc.log("debug", "%s ⇒ %s [%s]: %s", *message.SourceId, *message.DestinationId, *message.Namespace, *message.PayloadUtf8)

	err = binary.Write(cc.conn, binary.BigEndian, uint32(len(data)))
	if err != nil {
		return err
	}
	_, err = cc.conn.Write(data)
	return err
}

func (cc *CastConnection) receiveLoop() {
	for {
		var length uint32
		err := binary.Read(cc.conn, binary.BigEndian, &length)
		if err != nil {
			cc.log("error", "Failed to read packet length: %s", err)
			break
		}
		if length == 0 {
			cc.log("debug", "Empty packet received")
			continue
		}

		packet := make([]byte, length)
		i, err := io.ReadFull(cc.conn, packet)
		if err != nil {
			cc.log("error", "Failed to read packet: %s", err)
			break
		}

		if i != int(length) {
			cc.log("error", "Invalid packet size. Wanted: %d Read: %d", length, i)
			break
		}

		message := &api.CastMessage{}
		err = proto.Unmarshal(packet, message)
		if err != nil {
			cc.log("error", "Failed to unmarshal CastMessage: %s", err)
			break
		}

		cc.log("debug", "%s ⇐ %s [%s]: %+v", *message.DestinationId, *message.SourceId, *message.Namespace, *message.PayloadUtf8)

		var headers PayloadHeader
		err = json.Unmarshal([]byte(*message.PayloadUtf8), &headers)

		if err != nil {
			cc.log("error", "Failed to unmarshal message: %s", err)
			break
		}

		cc.handleMessage(message, &headers)
	}
}

func (cc *CastConnection) handleMessage(message *api.CastMessage, headers *PayloadHeader) {

	messageType, err := jsonparser.GetString([]byte(*message.PayloadUtf8), "type")
	if err != nil {
		cc.log("error", "Could not find 'type' key in response: %s\n", err)
		return
	}

	switch messageType {
	case "PING":
		if err := cc.send(pongHeader, *message.SourceId, *message.DestinationId, *message.Namespace); err != nil {
			cc.log("error", "Error sending message: %s\n", err)
		}
	default:
		for _, handler := range cc.messageHandlers {
			if handler(message) {
				return
			}
		}
	}
}

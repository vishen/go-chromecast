package net

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"golang.org/x/net/context"

	"github.com/barnybug/go-cast/api"
	"github.com/barnybug/go-cast/log"
	"github.com/gogo/protobuf/proto"
)

type Connection struct {
	conn     *tls.Conn
	channels []*Channel
}

func NewConnection() *Connection {
	return &Connection{
		conn:     nil,
		channels: make([]*Channel, 0),
	}
}

func (c *Connection) NewChannel(sourceId, destinationId, namespace string) *Channel {
	channel := NewChannel(c, sourceId, destinationId, namespace)
	c.channels = append(c.channels, channel)
	return channel
}

func (c *Connection) Connect(ctx context.Context, host net.IP, port int) error {
	var err error
	deadline, _ := ctx.Deadline()
	dialer := &net.Dialer{
		Deadline: deadline,
	}
	c.conn, err = tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", host, port), &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Errorf("Failed to connect to Chromecast: %s", err)
	}

	go c.ReceiveLoop()

	return nil
}

func (c *Connection) ReceiveLoop() {
	for {
		var length uint32
		err := binary.Read(c.conn, binary.BigEndian, &length)
		if err != nil {
			log.Printf("Failed to read packet length: %s", err)
			break
		}
		if length == 0 {
			log.Println("Empty packet received")
			continue
		}

		packet := make([]byte, length)
		i, err := io.ReadFull(c.conn, packet)
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

		log.Printf("%s ⇐ %s [%s]: %+v",
			*message.DestinationId, *message.SourceId, *message.Namespace, *message.PayloadUtf8)

		var headers PayloadHeaders
		err = json.Unmarshal([]byte(*message.PayloadUtf8), &headers)

		if err != nil {
			log.Printf("Failed to unmarshal message: %s", err)
			break
		}

		for _, channel := range c.channels {
			channel.Message(message, &headers)
		}
	}
}

func (c *Connection) Send(payload interface{}, sourceId, destinationId, namespace string) error {
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

	err = binary.Write(c.conn, binary.BigEndian, uint32(len(data)))
	if err != nil {
		return err
	}
	_, err = c.conn.Write(data)
	return err
}

func (c *Connection) Close() error {
	// TODO: graceful shutdown
	return c.conn.Close()
}

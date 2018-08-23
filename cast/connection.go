package cast

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/buger/jsonparser"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"

	pb "github.com/vishen/go-chromecast/cast/proto"
)

const (
	dialerTimeout   = time.Second * 30
	dialerKeepAlive = time.Second * 30
)

var (

	// Global request id
	requestID int
)

type Connection struct {
	conn *tls.Conn

	resultChanMap map[int]chan *pb.CastMessage

	debugging bool
	connected bool
}

func NewConnection(debugging bool) *Connection {
	c := &Connection{
		resultChanMap: map[int]chan *pb.CastMessage{},
		debugging:     debugging,
	}
	return c
}

func (c *Connection) Start(addr string, port int) error {
	if !c.connected {
		return c.connect(addr, port)
	}
	return nil
}

func (c *Connection) debug(message string, args ...interface{}) {
	if c.debugging {
		log.Printf("[connection] %s", fmt.Sprintf(message, args...))
	}
}

func (c *Connection) connect(addr string, port int) error {
	var err error
	dialer := &net.Dialer{
		Timeout:   dialerTimeout,
		KeepAlive: dialerKeepAlive,
	}
	c.conn, err = tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", addr, port), &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return errors.Wrapf(err, "unable to connect to chromecast at '%s:%d'", addr, port)
	}
	c.connected = true
	return nil
}

/*func (c *Connection) SendDefaultConn(ctx context.Context, payload Payload) (*pb.CastMessage, error) {
	return c.Send(ctx, payload, defaultSender, defaultRecv, namespaceConn)
}

func (c *Connection) SendDefaultRecv(ctx context.Context, payload Payload) (*pb.CastMessage, error) {
	return c.Send(ctx, payload, defaultSender, defaultRecv, namespaceRecv)
}

func (c *Connection) SendMediaConn(ctx context.Context, payload Payload, transportID string) (*pb.CastMessage, error) {
	return c.Send(ctx, payload, defaultSender, transportID, namespaceConn)
}

func (c *Connection) SendMediaConn(ctx context.Context, payload Payload, transportID string) (*pb.CastMessage, error) {
	return c.Send(ctx, payload, defaultSender, transportID, namespaceMedia)
}*/

func (c *Connection) Send(ctx context.Context, payload Payload, sourceID, destinationID, namespace string) (*pb.CastMessage, error) {

	// NOTE: Not concurrent safe, but currently only synchronous flow is possible
	// TODO(vishen): just make concurrent safe regardless of current flow
	requestID += 1
	payload.SetRequestId(requestID)

	resultChan := make(chan *pb.CastMessage, 1)
	c.resultChanMap[requestID] = resultChan
	defer func() {
		delete(c.resultChanMap, requestID)
	}()

	if err := c.send(payload, sourceID, destinationID, namespace); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChan:
		return result, nil
	}
}

func (c *Connection) send(payload Payload, sourceID, destinationID, namespace string) error {
	payloadJson, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "unable to marshal json payload")
	}
	payloadUtf8 := string(payloadJson)
	message := &pb.CastMessage{
		ProtocolVersion: pb.CastMessage_CASTV2_1_0.Enum(),
		SourceId:        &sourceID,
		DestinationId:   &destinationID,
		Namespace:       &namespace,
		PayloadType:     pb.CastMessage_STRING.Enum(),
		PayloadUtf8:     &payloadUtf8,
	}
	proto.SetDefaults(message)
	data, err := proto.Marshal(message)
	if err != nil {
		return errors.Wrap(err, "unable to marshal proto payload")
	}

	c.debug("%s -> %s [%s]: %s", sourceID, destinationID, namespace, payloadJson)

	if err := binary.Write(c.conn, binary.BigEndian, uint32(len(data))); err != nil {
		return errors.Wrap(err, "unable to write binary format")
	}
	if _, err := c.conn.Write(data); err != nil {
		return errors.Wrap(err, "unable to send data")
	}

	return nil
}

func (c *Connection) receiveLoop() {
	for {
		var length uint32
		if err := binary.Read(c.conn, binary.BigEndian, &length); err != nil {
			c.debug("failed to binary read payload: %v", err)
			break
		}
		if length == 0 {
			c.debug("empty payload received")
			continue
		}

		payload := make([]byte, length)
		i, err := io.ReadFull(c.conn, payload)
		if err != nil {
			c.debug("failed to read payload: %v", err)
			continue
		}

		if i != int(length) {
			c.debug("invalid payload, wanted: %d but read: %d", length, i)
			continue
		}

		message := &pb.CastMessage{}
		if err := proto.Unmarshal(payload, message); err != nil {
			c.debug("failed to unmarshal proto cast message '%s': %v", payload, err)
			continue
		}

		c.debug("%s <- %s [%s]: %s", *message.DestinationId, *message.SourceId, *message.Namespace, *message.PayloadUtf8)

		var headers PayloadHeader
		if err := json.Unmarshal([]byte(*message.PayloadUtf8), &headers); err != nil {
			c.debug("failed to unmarshal proto message header: %v", err)
			continue
		}

		c.handleMessage(message, &headers)
	}
}

func (c *Connection) handleMessage(message *pb.CastMessage, headers *PayloadHeader) {

	messageType, err := jsonparser.GetString([]byte(*message.PayloadUtf8), "type")
	if err != nil {
		c.debug("could not find 'type' key in response message %q: %s", *message.PayloadUtf8, err)
		return
	}

	switch messageType {
	case "PING":
		if err := c.send(&PongHeader, *message.SourceId, *message.DestinationId, *message.Namespace); err != nil {
			c.debug("unable to respond to 'PING': %v", err)
		}
	default:
		requestID, err := jsonparser.GetInt([]byte(*message.PayloadUtf8), "requestId")
		if err != nil {
			c.debug("unable to find 'requestId' in proto payload '%s': %v", *message.PayloadUtf8, err)
		}
		if resultChan, ok := c.resultChanMap[int(requestID)]; ok {
			resultChan <- message
		}
	}
}

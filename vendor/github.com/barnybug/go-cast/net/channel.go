package net

import (
	"sync/atomic"

	"golang.org/x/net/context"

	"github.com/barnybug/go-cast/api"
	"github.com/barnybug/go-cast/log"
)

type Channel struct {
	conn          *Connection
	sourceId      string
	DestinationId string
	namespace     string
	_ int32 
	requestId     int64
	inFlight      map[int]chan *api.CastMessage
	listeners     []channelListener
}

type channelListener struct {
	responseType string
	callback     func(*api.CastMessage)
}

type Payload interface {
	setRequestId(id int)
	getRequestId() int
}

func NewChannel(conn *Connection, sourceId, destinationId, namespace string) *Channel {
	return &Channel{
		conn:          conn,
		sourceId:      sourceId,
		DestinationId: destinationId,
		namespace:     namespace,
		listeners:     make([]channelListener, 0),
		inFlight:      make(map[int]chan *api.CastMessage),
	}
}

func (c *Channel) Message(message *api.CastMessage, headers *PayloadHeaders) {
	if *message.DestinationId != "*" && (*message.SourceId != c.DestinationId || *message.DestinationId != c.sourceId || *message.Namespace != c.namespace) {
		return
	}

	if headers.Type == "" {
		log.Errorf("Warning: No message type. Don't know what to do. headers: %v message:%v", headers, message)
		return
	}

	if headers.RequestId != nil && *headers.RequestId != 0 {
		if listener, ok := c.inFlight[*headers.RequestId]; ok {
			listener <- message
			delete(c.inFlight, *headers.RequestId)
		}
	}

	for _, listener := range c.listeners {
		if listener.responseType == headers.Type {
			listener.callback(message)
		}
	}
}

func (c *Channel) OnMessage(responseType string, cb func(*api.CastMessage)) {
	c.listeners = append(c.listeners, channelListener{responseType, cb})
}

func (c *Channel) Send(payload interface{}) error {
	return c.conn.Send(payload, c.sourceId, c.DestinationId, c.namespace)
}

func (c *Channel) Request(ctx context.Context, payload Payload) (*api.CastMessage, error) {
	requestId := int(atomic.AddInt64(&c.requestId, 1))

	payload.setRequestId(requestId)
	response := make(chan *api.CastMessage)
	c.inFlight[requestId] = response

	err := c.Send(payload)
	if err != nil {
		delete(c.inFlight, requestId)
		return nil, err
	}

	select {
	case reply := <-response:
		return reply, nil
	case <-ctx.Done():
		delete(c.inFlight, requestId)
		return nil, ctx.Err()
	}
}

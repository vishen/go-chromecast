package main

import (
	"fmt"
	"time"

	"github.com/buger/jsonparser"
	"github.com/vishen/go-chromecast/api"
)

var (
	// Global request id
	requestId int
)

type CastInterface struct {
	castConn *CastConnection

	sourceId      string
	destinationId string
	namespace     string

	resultChanMap map[int]chan *api.CastMessage
}

func NewCastInterface(castConnection *CastConnection, sourceId, destinationId, namespace string) *CastInterface {
	ci := &CastInterface{
		castConn:      castConnection,
		sourceId:      sourceId,
		destinationId: destinationId,
		namespace:     namespace,
		resultChanMap: map[int]chan *api.CastMessage{},
	}

	castConnection.addMessageHandler(ci.MessageHandler)

	return ci
}

func (ci *CastInterface) Send(payload Payload) error {
	// NOTE: Not concurrent safe, but currently only synchronous flow is possible
	requestId += 1
	payload.SetRequestId(requestId)
	return ci.castConn.send(payload, ci.sourceId, ci.destinationId, ci.namespace)
}

func (ci *CastInterface) SendAndWait(payload Payload) (*api.CastMessage, error) {

	// NOTE: Not concurrent safe, but currently only synchronous flow is possible
	requestId += 1
	payload.SetRequestId(requestId)

	resultChan := make(chan *api.CastMessage, 1)

	ci.resultChanMap[requestId] = resultChan
	defer func() {
		delete(ci.resultChanMap, requestId)
	}()

	if err := ci.castConn.send(payload, ci.sourceId, ci.destinationId, ci.namespace); err != nil {
		return nil, err
	}

	timer := time.NewTimer(time.Second * 10)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil, fmt.Errorf("Timeout waiting for result")
	case resultMessage := <-resultChan:
		return resultMessage, nil
	}
}

func (ci *CastInterface) MessageHandler(message *api.CastMessage) bool {
	requestId, err := jsonparser.GetInt([]byte(*message.PayloadUtf8), "requestId")
	if err != nil {
		return false
	}
	resultChan, ok := ci.resultChanMap[int(requestId)]
	if ok {
		resultChan <- message
		return true
	} else {

	}

	return false

	// 2017/11/21 13:00:24 [debug] * ⇐ f93eca59-a184-4c6b-926e-6e4009e1a69f [urn:x-cast:com.google.cast.media]: {"type":"MEDIA_STATUS","status":[{"mediaSessionId":4,"playbackRate":1,"playerState":"IDLE","currentTime":0,"supportedMediaCommands":15,"volume":{"level":1,"muted":false},"currentItemId":4,"idleReason":"FINISHED"}],"requestId":0}
}

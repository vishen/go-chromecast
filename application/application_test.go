package application_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vishen/go-chromecast/application"
	"github.com/vishen/go-chromecast/cast"
	mockCast "github.com/vishen/go-chromecast/cast/mock"
	pb "github.com/vishen/go-chromecast/cast/proto"
)

var mockAddr = "foo.bar"
var mockPort = 42

func TestApplicationStart(t *testing.T) {
	assertions := require.New(t)

	var recvChan chan *pb.CastMessage
	conn := &mockCast.Connection{}
	conn.On("SetMsgChan", mock.Anything).Run(func(args mock.Arguments) {
		argChan := args.Get(0).(chan *pb.CastMessage)
		recvChan = argChan
	})
	conn.On("SetDebug", true)
	conn.On("Start", mockAddr, mockPort).Return(nil)
	conn.On("Send", mock.Anything, mock.IsType(&cast.PayloadHeader{}), mock.Anything, mock.Anything, mock.Anything).Return(nil)
	conn.On("Send", mock.Anything, mock.IsType(&cast.ConnectHeader), mock.Anything, mock.Anything, mock.Anything).Return(nil)
	app := application.NewApplication(application.WithConnection(conn), application.WithDebug(true))
	assertions.NotNil(recvChan)
	go func() {
		payload := cast.GetStatusHeader
		payload.SetRequestId(2)
		payloadBytes, err := json.Marshal(&cast.ReceiverStatusResponse{PayloadHeader: payload})
		assertions.NoError(err)
		payloadString := string(payloadBytes)
		protocolVersion := pb.CastMessage_CASTV2_1_0
		payloadType := pb.CastMessage_STRING
		recvChan <- &pb.CastMessage{
			ProtocolVersion: &protocolVersion,
			PayloadType:     &payloadType,
			PayloadUtf8:     &payloadString,
			PayloadBinary:   payloadBytes,
		}
	}()
	assertions.NoError(app.Start(mockAddr, mockPort))
}

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

	recvChan := make(chan *pb.CastMessage, 5)
	conn := &mockCast.Conn{}
	conn.On("MsgChan").Return(recvChan)
	conn.On("Start", mockAddr, mockPort).Return(nil)
	conn.On("Send", mock.IsType(0), mock.IsType(&cast.PayloadHeader{}), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			payload := cast.GetStatusHeader
			payload.SetRequestId(args.Int(0))
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
		}).Return(nil)
	conn.On("Send", mock.IsType(0), mock.IsType(&cast.ConnectHeader), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	app := application.NewApplication(application.WithConnection(conn))
	assertions.NoError(app.Start(mockAddr, mockPort))
}

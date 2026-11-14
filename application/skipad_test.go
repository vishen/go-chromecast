package application_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/buger/jsonparser"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vishen/go-chromecast/application"
	"github.com/vishen/go-chromecast/cast"
	mockCast "github.com/vishen/go-chromecast/cast/mocks"
	pb "github.com/vishen/go-chromecast/cast/proto"
)

func castMessage(t *testing.T, payload any) *pb.CastMessage {
	t.Helper()
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)
	payloadString := string(payloadBytes)
	protocolVersion := pb.CastMessage_CASTV2_1_0
	payloadType := pb.CastMessage_STRING
	return &pb.CastMessage{
		ProtocolVersion: &protocolVersion,
		PayloadType:     &payloadType,
		PayloadUtf8:     &payloadString,
	}
}

// TestSkipadMediaClosed covers the receiver closing the session (e.g. the
// YouTube app crashing or relaunching) while Skipad is retrying. The CLOSE
// message clears the media state from the receive goroutine, and Skipad must
// return an error instead of dereferencing the now-nil media.
func TestSkipadMediaClosed(t *testing.T) {
	recvChan := make(chan *pb.CastMessage, 5)
	closeHandled := make(chan struct{})

	conn := &mockCast.Conn{}
	conn.On("MsgChan").Return(recvChan)
	conn.On("Start", mockAddr, mockPort).Return(nil)
	conn.On("Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			requestID := args.Int(0)
			payloadBytes, err := json.Marshal(args.Get(1))
			require.NoError(t, err)
			payloadType, _ := jsonparser.GetString(payloadBytes, "type")
			namespace := args.String(4)

			switch {
			case payloadType == "GET_STATUS" && namespace == "urn:x-cast:com.google.cast.receiver":
				resp := cast.ReceiverStatusResponse{PayloadHeader: cast.PayloadHeader{Type: "RECEIVER_STATUS", RequestId: requestID}}
				resp.Status.Applications = []cast.Application{{AppId: "233637DE", TransportId: "transport-1"}}
				recvChan <- castMessage(t, resp)
			case payloadType == "GET_STATUS" && namespace == "urn:x-cast:com.google.cast.media":
				recvChan <- castMessage(t, cast.MediaStatusResponse{
					PayloadHeader: cast.PayloadHeader{Type: "MEDIA_STATUS", RequestId: requestID},
					Status:        []cast.Media{{MediaSessionId: 1, CustomData: cast.CustomData{PlayerState: 1081}}},
				})
			case payloadType == "SKIP_AD":
				// The receiver goes away instead of skipping the ad. Wait until
				// the CLOSE has been processed so the test is deterministic.
				recvChan <- castMessage(t, cast.PayloadHeader{Type: "CLOSE"})
				select {
				case <-closeHandled:
				case <-time.After(5 * time.Second):
					t.Error("timed out waiting for CLOSE to be handled")
				}
			}
		}).Return(nil)

	app := application.NewApplication(
		application.WithConnection(conn),
		application.WithCacheDisabled(true),
		application.WithSkipadSleep(time.Millisecond),
		application.WithSkipadRetries(5),
	)
	app.AddMessageFunc(func(msg *pb.CastMessage) {
		if msgType, _ := jsonparser.GetString([]byte(msg.GetPayloadUtf8()), "type"); msgType == "CLOSE" {
			close(closeHandled)
		}
	})
	require.NoError(t, app.Start(mockAddr, mockPort))
	require.NotNil(t, app.Media(), "media should be loaded by Start")

	var err error
	require.NotPanics(t, func() { err = app.Skipad() })
	require.ErrorIs(t, err, application.ErrNoMediaSkip)
}

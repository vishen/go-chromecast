package application_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vishen/go-chromecast/application"
	"github.com/vishen/go-chromecast/cast"
	mockCast "github.com/vishen/go-chromecast/cast/mocks"
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

// replayConn returns a connection to a device that is running the default
// media receiver with a media in the given player state, or with no media
// if the player state is empty. Everything sent that isn't a status request
// ends up in sent.
func replayConn(t *testing.T, playerState *string, sent *[]cast.Payload) *mockCast.Conn {
	recvChan := make(chan *pb.CastMessage, 5)
	reply := func(requestID int, response interface{}) {
		payloadBytes, err := json.Marshal(response)
		require.NoError(t, err)
		payloadString := string(payloadBytes)
		protocolVersion := pb.CastMessage_CASTV2_1_0
		payloadType := pb.CastMessage_STRING
		recvChan <- &pb.CastMessage{
			ProtocolVersion: &protocolVersion,
			PayloadType:     &payloadType,
			PayloadUtf8:     &payloadString,
			PayloadBinary:   payloadBytes,
		}
	}

	conn := &mockCast.Conn{}
	conn.On("MsgChan").Return(recvChan)
	conn.On("Start", mockAddr, mockPort).Return(nil)
	conn.On("Send", mock.IsType(0), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			payload := args.Get(1).(cast.Payload)
			header, isHeader := payload.(*cast.PayloadHeader)
			if !isHeader || header.Type != cast.GetStatusHeader.Type {
				if !isHeader || header.Type != cast.ConnectHeader.Type {
					*sent = append(*sent, payload)
				}
				return
			}

			responseHeader := cast.GetStatusHeader
			responseHeader.SetRequestId(args.Int(0))
			if args.String(3) == "receiver-0" {
				response := &cast.ReceiverStatusResponse{PayloadHeader: responseHeader}
				response.Status.Applications = []cast.Application{{AppId: "CC1AD845", TransportId: "transport-0"}}
				reply(args.Int(0), response)
				return
			}
			response := &cast.MediaStatusResponse{PayloadHeader: responseHeader}
			if *playerState != "" {
				response.Status = []cast.Media{{
					MediaSessionId: 7,
					PlayerState:    *playerState,
					CurrentTime:    42,
					Media:          cast.MediaItem{ContentId: "http://foo.bar/video.mp4", ContentType: "video/mp4", StreamType: "BUFFERED"},
				}}
			}
			reply(args.Int(0), response)
		}).Return(nil)
	return conn
}

func TestApplicationReplay(t *testing.T) {
	t.Run("seeks to the start when the media is still loaded", func(t *testing.T) {
		assertions := require.New(t)

		var sent []cast.Payload
		playerState := "PLAYING"
		app := application.NewApplication(application.WithConnection(replayConn(t, &playerState, &sent)))
		assertions.NoError(app.Start(mockAddr, mockPort))
		assertions.NoError(app.Replay())

		assertions.Len(sent, 1)
		seek, ok := sent[0].(*cast.MediaHeader)
		assertions.True(ok)
		assertions.Equal(cast.SeekHeader.Type, seek.Type)
		assertions.Equal(7, seek.MediaSessionId)
		assertions.Equal(float32(0), seek.CurrentTime)
		assertions.Equal("PLAYBACK_START", seek.ResumeState)
	})

	t.Run("loads the media again when it is idle", func(t *testing.T) {
		assertions := require.New(t)

		var sent []cast.Payload
		playerState := "IDLE"
		app := application.NewApplication(application.WithConnection(replayConn(t, &playerState, &sent)))
		assertions.NoError(app.Start(mockAddr, mockPort))
		assertions.NoError(app.Replay())

		assertions.Len(sent, 1)
		load, ok := sent[0].(*cast.LoadMediaCommand)
		assertions.True(ok)
		assertions.Equal("http://foo.bar/video.mp4", load.Media.ContentId)
	})

	t.Run("loads the media again when it has finished and there is no status", func(t *testing.T) {
		assertions := require.New(t)

		var sent []cast.Payload
		playerState := "PLAYING"
		app := application.NewApplication(application.WithConnection(replayConn(t, &playerState, &sent)))
		assertions.NoError(app.Start(mockAddr, mockPort))

		// The media finishes: the device doesn't report it anymore.
		playerState = ""
		assertions.NoError(app.Replay())

		assertions.Len(sent, 1)
		load, ok := sent[0].(*cast.LoadMediaCommand)
		assertions.True(ok)
		assertions.Equal(cast.LoadHeader.Type, load.Type)
		assertions.True(load.Autoplay)
		assertions.Equal("http://foo.bar/video.mp4", load.Media.ContentId)
		assertions.Equal("video/mp4", load.Media.ContentType)
	})

	t.Run("fails when there is no media", func(t *testing.T) {
		conn := &mockCast.Conn{}
		conn.On("MsgChan").Return(make(chan *pb.CastMessage))
		app := application.NewApplication(application.WithConnection(conn))
		require.Equal(t, application.ErrNoMediaReplay, app.Replay())
	})
}

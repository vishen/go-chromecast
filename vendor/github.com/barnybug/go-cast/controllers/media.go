package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/barnybug/go-cast/api"
	"github.com/barnybug/go-cast/events"
	"github.com/barnybug/go-cast/log"
	"github.com/barnybug/go-cast/net"
)

type MediaController struct {
	interval       time.Duration
	channel        *net.Channel
	eventsCh       chan events.Event
	DestinationID  string
	MediaSessionID int
}

const NamespaceMedia = "urn:x-cast:com.google.cast.media"

var getMediaStatus = net.PayloadHeaders{Type: "GET_STATUS"}

var commandMediaPlay = net.PayloadHeaders{Type: "PLAY"}
var commandMediaPause = net.PayloadHeaders{Type: "PAUSE"}
var commandMediaStop = net.PayloadHeaders{Type: "STOP"}
var commandMediaLoad = net.PayloadHeaders{Type: "LOAD"}

type MediaCommand struct {
	net.PayloadHeaders
	MediaSessionID int `json:"mediaSessionId"`
}

type LoadMediaCommand struct {
	net.PayloadHeaders
	Media       MediaItem   `json:"media"`
	CurrentTime int         `json:"currentTime"`
	Autoplay    bool        `json:"autoplay"`
	CustomData  interface{} `json:"customData"`
}

type MediaItem struct {
	ContentId   string `json:"contentId"`
	StreamType  string `json:"streamType"`
	ContentType string `json:"contentType"`
}

type MediaStatusMedia struct {
	ContentId   string  `json:"contentId"`
	StreamType  string  `json:"streamType"`
	ContentType string  `json:"contentType"`
	Duration    float64 `json:"duration"`
}

func NewMediaController(conn *net.Connection, eventsCh chan events.Event, sourceId, destinationID string) *MediaController {
	controller := &MediaController{
		channel:       conn.NewChannel(sourceId, destinationID, NamespaceMedia),
		eventsCh:      eventsCh,
		DestinationID: destinationID,
	}

	controller.channel.OnMessage("MEDIA_STATUS", controller.onStatus)

	return controller
}

func (c *MediaController) SetDestinationID(id string) {
	c.channel.DestinationId = id
	c.DestinationID = id
}

func (c *MediaController) sendEvent(event events.Event) {
	select {
	case c.eventsCh <- event:
	default:
		log.Printf("Dropped event: %#v", event)
	}
}

func (c *MediaController) onStatus(message *api.CastMessage) {
	response, err := c.parseStatus(message)
	if err != nil {
		log.Errorf("Error parsing status: %s", err)
	}

	for _, status := range response.Status {
		c.sendEvent(*status)
	}
}

func (c *MediaController) parseStatus(message *api.CastMessage) (*MediaStatusResponse, error) {
	response := &MediaStatusResponse{}

	err := json.Unmarshal([]byte(*message.PayloadUtf8), response)

	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal status message:%s - %s", err, *message.PayloadUtf8)
	}

	for _, status := range response.Status {
		c.MediaSessionID = status.MediaSessionID
	}

	return response, nil
}

type MediaStatusResponse struct {
	net.PayloadHeaders
	Status []*MediaStatus `json:"status,omitempty"`
}

type MediaStatus struct {
	net.PayloadHeaders
	MediaSessionID         int                    `json:"mediaSessionId"`
	PlaybackRate           float64                `json:"playbackRate"`
	PlayerState            string                 `json:"playerState"`
	CurrentTime            float64                `json:"currentTime"`
	SupportedMediaCommands int                    `json:"supportedMediaCommands"`
	Volume                 *Volume                `json:"volume,omitempty"`
	Media                  *MediaStatusMedia      `json:"media"`
	CustomData             map[string]interface{} `json:"customData"`
	RepeatMode             string                 `json:"repeatMode"`
	IdleReason             string                 `json:"idleReason"`
}

func (c *MediaController) Start(ctx context.Context) error {
	_, err := c.GetStatus(ctx)
	return err
}

func (c *MediaController) GetStatus(ctx context.Context) (*MediaStatusResponse, error) {
	message, err := c.channel.Request(ctx, &getMediaStatus)
	if err != nil {
		return nil, fmt.Errorf("Failed to get receiver status: %s", err)
	}

	return c.parseStatus(message)
}

func (c *MediaController) Play(ctx context.Context) (*api.CastMessage, error) {
	message, err := c.channel.Request(ctx, &MediaCommand{commandMediaPlay, c.MediaSessionID})
	if err != nil {
		return nil, fmt.Errorf("Failed to send play command: %s", err)
	}
	return message, nil
}

func (c *MediaController) Pause(ctx context.Context) (*api.CastMessage, error) {
	message, err := c.channel.Request(ctx, &MediaCommand{commandMediaPause, c.MediaSessionID})
	if err != nil {
		return nil, fmt.Errorf("Failed to send pause command: %s", err)
	}
	return message, nil
}

func (c *MediaController) Stop(ctx context.Context) (*api.CastMessage, error) {
	if c.MediaSessionID == 0 {
		// no current session to stop
		return nil, nil
	}
	message, err := c.channel.Request(ctx, &MediaCommand{commandMediaStop, c.MediaSessionID})
	if err != nil {
		return nil, fmt.Errorf("Failed to send stop command: %s", err)
	}
	return message, nil
}

func (c *MediaController) LoadMedia(ctx context.Context, media MediaItem, currentTime int, autoplay bool, customData interface{}) (*api.CastMessage, error) {
	message, err := c.channel.Request(ctx, &LoadMediaCommand{
		PayloadHeaders: commandMediaLoad,
		Media:          media,
		CurrentTime:    currentTime,
		Autoplay:       autoplay,
		CustomData:     customData,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to send load command: %s", err)
	}

	response := &net.PayloadHeaders{}
	err = json.Unmarshal([]byte(*message.PayloadUtf8), response)
	if err != nil {
		return nil, err
	}
	if response.Type == "LOAD_FAILED" {
		return nil, errors.New("Load media failed")
	}

	return message, nil
}

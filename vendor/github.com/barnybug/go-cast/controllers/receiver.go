package controllers

import (
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/barnybug/go-cast/api"
	"github.com/barnybug/go-cast/events"
	"github.com/barnybug/go-cast/log"
	"github.com/barnybug/go-cast/net"
)

type ReceiverController struct {
	interval time.Duration
	channel  *net.Channel
	eventsCh chan events.Event
	status   *ReceiverStatus
}

var getStatus = net.PayloadHeaders{Type: "GET_STATUS"}
var commandLaunch = net.PayloadHeaders{Type: "LAUNCH"}
var commandStop = net.PayloadHeaders{Type: "STOP"}

func NewReceiverController(conn *net.Connection, eventsCh chan events.Event, sourceId, destinationId string) *ReceiverController {
	controller := &ReceiverController{
		channel:  conn.NewChannel(sourceId, destinationId, "urn:x-cast:com.google.cast.receiver"),
		eventsCh: eventsCh,
	}

	controller.channel.OnMessage("RECEIVER_STATUS", controller.onStatus)

	return controller
}

func (c *ReceiverController) sendEvent(event events.Event) {
	select {
	case c.eventsCh <- event:
	default:
		log.Printf("Dropped event: %#v", event)
	}
}

func (c *ReceiverController) onStatus(message *api.CastMessage) {
	response := &StatusResponse{}
	err := json.Unmarshal([]byte(*message.PayloadUtf8), response)
	if err != nil {
		log.Errorf("Failed to unmarshal status message:%s - %s", err, *message.PayloadUtf8)
		return
	}

	previous := map[string]*ApplicationSession{}
	if c.status != nil {
		for _, app := range c.status.Applications {
			previous[*app.AppID] = app
		}
	}

	c.status = response.Status
	vol := response.Status.Volume
	c.sendEvent(events.StatusUpdated{Level: *vol.Level, Muted: *vol.Muted})

	for _, app := range response.Status.Applications {
		if _, ok := previous[*app.AppID]; ok {
			// Already running
			delete(previous, *app.AppID)
			continue
		}
		event := events.AppStarted{
			AppID:       *app.AppID,
			DisplayName: *app.DisplayName,
			StatusText:  *app.StatusText,
		}
		c.sendEvent(event)
	}

	// Stopped apps
	for _, app := range previous {
		event := events.AppStopped{
			AppID:       *app.AppID,
			DisplayName: *app.DisplayName,
			StatusText:  *app.StatusText,
		}
		c.sendEvent(event)
	}
}

type StatusResponse struct {
	net.PayloadHeaders
	Status *ReceiverStatus `json:"status,omitempty"`
}

type ReceiverStatus struct {
	net.PayloadHeaders
	Applications []*ApplicationSession `json:"applications"`
	Volume       *Volume               `json:"volume,omitempty"`
}

type LaunchRequest struct {
	net.PayloadHeaders
	AppId string `json:"appId"`
}

func (s *ReceiverStatus) GetSessionByNamespace(namespace string) *ApplicationSession {
	for _, app := range s.Applications {
		for _, ns := range app.Namespaces {
			if ns.Name == namespace {
				return app
			}
		}
	}
	return nil
}

func (s *ReceiverStatus) GetSessionByAppId(appId string) *ApplicationSession {
	for _, app := range s.Applications {
		if *app.AppID == appId {
			return app
		}
	}
	return nil
}

type ApplicationSession struct {
	AppID       *string      `json:"appId,omitempty"`
	DisplayName *string      `json:"displayName,omitempty"`
	Namespaces  []*Namespace `json:"namespaces"`
	SessionID   *string      `json:"sessionId,omitempty"`
	StatusText  *string      `json:"statusText,omitempty"`
	TransportId *string      `json:"transportId,omitempty"`
}

type Namespace struct {
	Name string `json:"name"`
}

type Volume struct {
	Level *float64 `json:"level,omitempty"`
	Muted *bool    `json:"muted,omitempty"`
}

func (c *ReceiverController) Start(ctx context.Context) error {
	// noop
	return nil
}

func (c *ReceiverController) GetStatus(ctx context.Context) (*ReceiverStatus, error) {
	message, err := c.channel.Request(ctx, &getStatus)
	if err != nil {
		return nil, fmt.Errorf("Failed to get receiver status: %s", err)
	}

	response := &StatusResponse{}
	err = json.Unmarshal([]byte(*message.PayloadUtf8), response)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal status message: %s - %s", err, *message.PayloadUtf8)
	}

	return response.Status, nil
}

func (c *ReceiverController) SetVolume(ctx context.Context, volume *Volume) (*api.CastMessage, error) {
	return c.channel.Request(ctx, &ReceiverStatus{
		PayloadHeaders: net.PayloadHeaders{Type: "SET_VOLUME"},
		Volume:         volume,
	})
}

func (c *ReceiverController) GetVolume(ctx context.Context) (*Volume, error) {
	status, err := c.GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	return status.Volume, err
}

func (c *ReceiverController) LaunchApp(ctx context.Context, appId string) (*ReceiverStatus, error) {
	message, err := c.channel.Request(ctx, &LaunchRequest{
		PayloadHeaders: commandLaunch,
		AppId:          appId,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed sending request: %s", err)
	}

	response := &StatusResponse{}
	err = json.Unmarshal([]byte(*message.PayloadUtf8), response)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal status message: %s - %s", err, *message.PayloadUtf8)
	}
	return response.Status, nil
}

func (c *ReceiverController) QuitApp(ctx context.Context) (*api.CastMessage, error) {
	return c.channel.Request(ctx, &commandStop)
}
